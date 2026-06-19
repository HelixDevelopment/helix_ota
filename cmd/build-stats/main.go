// Build-resource stats tracker per §11.4.24.
//
// Samples host resources during a build process (RSS, CPU%, load average,
// disk read/write) at 5s intervals, computes min/max/mean/p95 per metric,
// appends to a TSV registry, and generates a Markdown report.
//
// Usage:
//
//	build-stats start              # starts background sampler, writes PID
//	build-stats stop               # stops sampler, computes stats, appends to TSV
//	build-stats report             # generates docs/Stats.md from TSV
//	build-stats serve -- <cmd...>  # wraps a command with resource tracking
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	pidFileName = "sampler.pid"
	runDirLink  = "current_run"
	samplesFile = "samples.tsv"
	summaryFile = "summary.json"

	// sampleInterval is the sampling period.
	sampleInterval = 5 * time.Second

	// Column counts for TSV parsing.
	headerColCount   = 8
	registryColCount = 23
)

var (
	// tsvFile is the path to the build stats registry TSV (var for test override).
	tsvFile string

	// dataDir is the build-stats data directory (resolved from project root).
	dataDir string

	// projectRoot is the git project root, lazily resolved.
	projectRoot string
)

// initPaths resolves data paths relative to the git project root.
func initPaths() {
	// Always re-resolve projectRoot to adapt to CWD
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		cwd, _ := os.Getwd()
		projectRoot = cwd
	} else {
		projectRoot = strings.TrimSpace(string(out))
	}
	tsvFile = filepath.Join(projectRoot, "qa-results", "build-stats", "build_stats.tsv")
	dataDir = filepath.Join(projectRoot, "qa-results", "build-stats")
}

// Sample holds one metrics snapshot at a point in time.
type Sample struct {
	Timestamp int64   `json:"ts"`        // UnixMilli
	RSSMB     int64   `json:"rss_mb"`    // System used memory, MB
	CPU       float64 `json:"cpu"`       // Total CPU usage across all processes, %
	Load1     float64 `json:"load_1"`    // Load average 1 min
	Load5     float64 `json:"load_5"`    // Load average 5 min
	Load15    float64 `json:"load_15"`   // Load average 15 min
	DiskR     int64   `json:"disk_r_mb"` // Cumulative disk read, MB (pageins*syspagesize)
	DiskW     int64   `json:"disk_w_mb"` // Cumulative disk write, MB (pageouts*syspagesize)
}

// BuildRecord holds computed stats for one build run.
type BuildRecord struct {
	BuildID   string `json:"build_id"`  // git describe --always --dirty
	Timestamp string `json:"timestamp"` // ISO 8601
	Status    string `json:"status"`    // SUCCESS / FAIL / UNKNOWN

	MinRSS  float64 `json:"min_rss"`
	MaxRSS  float64 `json:"max_rss"`
	MeanRSS float64 `json:"mean_rss"`
	P95RSS  float64 `json:"p95_rss"`

	MinCPU  float64 `json:"min_cpu"`
	MaxCPU  float64 `json:"max_cpu"`
	MeanCPU float64 `json:"mean_cpu"`
	P95CPU  float64 `json:"p95_cpu"`

	MinLoad  float64 `json:"min_load"`
	MaxLoad  float64 `json:"max_load"`
	MeanLoad float64 `json:"mean_load"`
	P95Load  float64 `json:"p95_load"`

	MinDiskR  float64 `json:"min_disk_r"`
	MaxDiskR  float64 `json:"max_disk_r"`
	MeanDiskR float64 `json:"mean_disk_r"`
	P95DiskR  float64 `json:"p95_disk_r"`

	MinDiskW  float64 `json:"min_disk_w"`
	MaxDiskW  float64 `json:"max_disk_w"`
	MeanDiskW float64 `json:"mean_disk_w"`
	P95DiskW  float64 `json:"p95_disk_w"`
}

// ---------------------------------------------------------------------------
// Platform-specific metric collectors

func getUsedMemoryMB() int64 {
	switch runtime.GOOS {
	case "darwin":
		return getMemoryDarwin()
	case "linux":
		return getMemoryLinux()
	default:
		return 0
	}
}

func getMemoryDarwin() int64 {
	// Total memory
	totalOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	total, _ := strconv.ParseInt(strings.TrimSpace(string(totalOut)), 10, 64)
	if total == 0 {
		return 0
	}

	// vm_stat for page counts
	vmOut, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}

	// Parse vm_stat: page size, then pages by category
	var pageSize int64 = 16384 // default Apple Silicon
	var pagesActive, pagesWired int64

	for _, line := range strings.Split(string(vmOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "page size of") {
			// "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
			ps, err := extractPageSize(line)
			if err == nil && ps > 0 {
				pageSize = ps
			}
			continue
		}
		if strings.HasPrefix(line, "Pages active:") {
			pagesActive = extractInt(line)
		} else if strings.HasPrefix(line, "Pages wired down:") {
			pagesWired = extractInt(line)
		}
	}

	usedPages := pagesActive + pagesWired
	usedBytes := usedPages * pageSize
	return usedBytes / (1024 * 1024) // MB
}

func getMemoryLinux() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var totalKB, freeKB, buffersKB, cachedKB int64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			totalKB = extractKB(line)
		case strings.HasPrefix(line, "MemFree:"):
			freeKB = extractKB(line)
		case strings.HasPrefix(line, "Buffers:"):
			buffersKB = extractKB(line)
		case strings.HasPrefix(line, "Cached:"):
			cachedKB = extractKB(line)
		}
	}
	if totalKB == 0 {
		return 0
	}
	usedKB := totalKB - freeKB - buffersKB - cachedKB
	return usedKB / 1024 // MB
}

// getTotalCPU returns the total CPU% across all processes.
func getTotalCPU() float64 {
	var out []byte
	var err error
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err = exec.Command("ps", "-A", "-o", "%cpu").Output()
	default:
		return 0
	}
	if err != nil {
		return 0
	}
	var total float64
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "%CPU" || line == "%cpu" {
			continue
		}
		v, err := strconv.ParseFloat(line, 64)
		if err == nil {
			total += v
		}
	}
	return total
}

// getLoadAverages returns 1, 5, 15 min load averages.
func getLoadAverages() (float64, float64, float64) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return 0, 0, 0
		}
		// Format: "{ 1.23 4.56 7.89 }"
		s := strings.TrimSpace(string(out))
		s = strings.Trim(s, "{} ")
		parts := strings.Fields(s)
		if len(parts) >= 3 {
			l1, _ := strconv.ParseFloat(parts[0], 64)
			l5, _ := strconv.ParseFloat(parts[1], 64)
			l15, _ := strconv.ParseFloat(parts[2], 64)
			return l1, l5, l15
		}
	case "linux":
		data, err := os.ReadFile("/proc/loadavg")
		if err == nil {
			parts := strings.Fields(string(data))
			if len(parts) >= 3 {
				l1, _ := strconv.ParseFloat(parts[0], 64)
				l5, _ := strconv.ParseFloat(parts[1], 64)
				l15, _ := strconv.ParseFloat(parts[2], 64)
				return l1, l5, l15
			}
		}
	}
	return 0, 0, 0
}

// getDiskIO returns cumulative pageins and pageouts in MB.
func getDiskIO() (int64, int64) {
	switch runtime.GOOS {
	case "darwin":
		// Get page size
		psOut, err := exec.Command("sysctl", "-n", "hw.pagesize").Output()
		if err != nil {
			return 0, 0
		}
		pageSize, _ := strconv.ParseInt(strings.TrimSpace(string(psOut)), 10, 64)
		if pageSize <= 0 {
			pageSize = 16384
		}

		vmOut, err := exec.Command("vm_stat").Output()
		if err != nil {
			return 0, 0
		}
		var pageins, pageouts int64
		for _, line := range strings.Split(string(vmOut), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "pageins:") {
				pageins = extractInt(line)
			} else if strings.HasPrefix(line, "pageouts:") {
				pageouts = extractInt(line)
			}
		}
		// Convert pages to MB
		return (pageins * pageSize) / (1024 * 1024), (pageouts * pageSize) / (1024 * 1024)
	case "linux":
		data, err := os.ReadFile("/proc/diskstats")
		if err != nil {
			return 0, 0
		}
		var readSectors, writeSectors int64
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			// Format: major minor name rio rmerge rsect ruse wio wmerge wsect wuse ...
			if len(fields) >= 10 {
				// Count only "main" devices (sdX, nvmeXnY, vdX) not partitions
				name := fields[2]
				if isMainDevice(name) {
					rs, _ := strconv.ParseInt(fields[5], 10, 64)
					ws, _ := strconv.ParseInt(fields[9], 10, 64)
					readSectors += rs
					writeSectors += ws
				}
			}
		}
		// 1 sector = 512 bytes
		return (readSectors * 512) / (1024 * 1024), (writeSectors * 512) / (1024 * 1024)
	}
	return 0, 0
}

func isMainDevice(name string) bool {
	// Match sdX, nvmeXnY, vdX — main block devices, NOT partition names.
	// sdX: exactly sd + one letter (sda, sdb, ...)
	if len(name) == 3 && (strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "vd")) {
		c := name[2]
		return c >= 'a' && c <= 'z'
	}
	// nvmeXnY: prefix nvme, then digits + 'n' + digits (no 'p' for partition)
	if strings.HasPrefix(name, "nvme") {
		return !strings.Contains(name, "p") // p partitions like nvme0n1p1
	}
	return false
}

// getBuildID returns the current git description.
func getBuildID() string {
	out, err := exec.Command("git", "describe", "--always", "--dirty").Output()
	if err != nil {
		return time.Now().Format("20060102T150405Z")
	}
	return strings.TrimSpace(string(out))
}

// ---------------------------------------------------------------------------
// Helper functions

func extractInt(line string) int64 {
	// Extracts the first integer from "Key:    12345." or "Key: 12345" or "Pages free:    12345."
	parts := strings.Fields(line)
	for _, p := range parts {
		p = strings.TrimRight(p, ".")
		v, err := strconv.ParseInt(p, 10, 64)
		if err == nil && v > 0 {
			return v
		}
	}
	return 0
}

func extractKB(line string) int64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	v, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func extractPageSize(line string) (int64, error) {
	// "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
	idx := strings.Index(line, "page size of")
	if idx < 0 {
		return 0, fmt.Errorf("not found")
	}
	rest := line[idx:]
	parts := strings.Fields(rest)
	if len(parts) < 4 {
		return 0, fmt.Errorf("too few fields")
	}
	return strconv.ParseInt(parts[3], 10, 64)
}

func p95Of(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	idx := int(math.Ceil(0.95*float64(len(sorted))) - 1)
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func meanOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func minOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func minI(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxI(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func meanI(vals []int64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

func p95I(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(0.95*float64(len(sorted))) - 1)
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

// ---------------------------------------------------------------------------
// daemon — the background sampler process

func runDaemon(runDir string) {
	pidPath := filepath.Join(runDir, pidFileName)
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot write PID file: %v\n", err)
		os.Exit(1)
	}

	samplePath := filepath.Join(runDir, samplesFile)
	f, err := os.Create(samplePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create samples file: %v\n", err)
		os.Exit(1)
	}
	// TSV header
	fmt.Fprintln(f, "ts\trss_mb\tcpu_pct\tload1\tload5\tload15\tdisk_r_mb\tdisk_w_mb")

	// Signal handler for graceful stop
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	// Initial sample
	sample := collectSample()
	if sample != nil {
		fmt.Fprintf(f, "%d\t%d\t%.1f\t%.2f\t%.2f\t%.2f\t%d\t%d\n",
			sample.Timestamp, sample.RSSMB, sample.CPU,
			sample.Load1, sample.Load5, sample.Load15,
			sample.DiskR, sample.DiskW)
		f.Sync()
	}

	for {
		select {
		case <-ticker.C:
			sample := collectSample()
			if sample != nil {
				fmt.Fprintf(f, "%d\t%d\t%.1f\t%.2f\t%.2f\t%.2f\t%d\t%d\n",
					sample.Timestamp, sample.RSSMB, sample.CPU,
					sample.Load1, sample.Load5, sample.Load15,
					sample.DiskR, sample.DiskW)
				f.Sync()
			}
		case <-sigCh:
			// Flush and close, then compute stats
			f.Close()
			computeAndSave(runDir, samplePath)
			os.Remove(pidPath)
			return
		}
	}
}

func collectSample() *Sample {
	s := &Sample{
		Timestamp: time.Now().UnixMilli(),
		RSSMB:     getUsedMemoryMB(),
		CPU:       getTotalCPU(),
	}
	s.Load1, s.Load5, s.Load15 = getLoadAverages()
	s.DiskR, s.DiskW = getDiskIO()
	return s
}

func computeAndSave(runDir, samplePath string) {
	buildID := getBuildID()
	now := time.Now().UTC().Format(time.RFC3339)

	samples := readSamples(samplePath)
	if len(samples) == 0 {
		// No samples means the build was too short; record UNKNOWN
		rec := &BuildRecord{
			BuildID:   buildID,
			Timestamp: now,
			Status:    "UNKNOWN",
		}
		appendRecord(rec)
		saveSummary(runDir, rec)
		return
	}

	rec := computeStats(samples, buildID, now)

	// Determine status from sample count
	rec.Status = "SUCCESS"
	if len(samples) < 2 {
		rec.Status = "UNKNOWN"
	}

	appendRecord(rec)
	saveSummary(runDir, rec)
}

func readSamples(path string) []*Sample {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var samples []*Sample
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || lineNo == 1 {
			// Skip header
			if lineNo == 1 {
				continue
			}
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < headerColCount {
			continue
		}
		s := &Sample{}
		ts, _ := strconv.ParseInt(fields[0], 10, 64)
		s.Timestamp = ts
		s.RSSMB, _ = strconv.ParseInt(fields[1], 10, 64)
		s.CPU, _ = strconv.ParseFloat(fields[2], 64)
		s.Load1, _ = strconv.ParseFloat(fields[3], 64)
		s.Load5, _ = strconv.ParseFloat(fields[4], 64)
		s.Load15, _ = strconv.ParseFloat(fields[5], 64)
		s.DiskR, _ = strconv.ParseInt(fields[6], 10, 64)
		s.DiskW, _ = strconv.ParseInt(fields[7], 10, 64)
		samples = append(samples, s)
	}
	return samples
}

func computeStats(samples []*Sample, buildID, timestamp string) *BuildRecord {
	rec := &BuildRecord{
		BuildID:   buildID,
		Timestamp: timestamp,
		Status:    "SUCCESS",
	}

	// Memory
	var rssVals []float64
	for _, s := range samples {
		rssVals = append(rssVals, float64(s.RSSMB))
	}
	rec.MinRSS = minOf(rssVals)
	rec.MaxRSS = maxOf(rssVals)
	rec.MeanRSS = meanOf(rssVals)
	rec.P95RSS = p95Of(rssVals)

	// CPU
	var cpuVals []float64
	for _, s := range samples {
		cpuVals = append(cpuVals, s.CPU)
	}
	rec.MinCPU = minOf(cpuVals)
	rec.MaxCPU = maxOf(cpuVals)
	rec.MeanCPU = meanOf(cpuVals)
	rec.P95CPU = p95Of(cpuVals)

	// Load
	var loadVals []float64
	for _, s := range samples {
		loadVals = append(loadVals, s.Load1)
	}
	rec.MinLoad = minOf(loadVals)
	rec.MaxLoad = maxOf(loadVals)
	rec.MeanLoad = meanOf(loadVals)
	rec.P95Load = p95Of(loadVals)

	// Disk I/O: compute per-interval deltas
	if len(samples) >= 2 {
		var diskRVals, diskWVals []float64
		for i := 1; i < len(samples); i++ {
			dr := float64(samples[i].DiskR - samples[i-1].DiskR)
			dw := float64(samples[i].DiskW - samples[i-1].DiskW)
			if dr >= 0 {
				diskRVals = append(diskRVals, dr)
			}
			if dw >= 0 {
				diskWVals = append(diskWVals, dw)
			}
		}
		if len(diskRVals) > 0 {
			rec.MinDiskR = minOf(diskRVals)
			rec.MaxDiskR = maxOf(diskRVals)
			rec.MeanDiskR = meanOf(diskRVals)
			rec.P95DiskR = p95Of(diskRVals)
		}
		if len(diskWVals) > 0 {
			rec.MinDiskW = minOf(diskWVals)
			rec.MaxDiskW = maxOf(diskWVals)
			rec.MeanDiskW = meanOf(diskWVals)
			rec.P95DiskW = p95Of(diskWVals)
		}
	}

	return rec
}

func appendRecord(rec *BuildRecord) {
	dir := filepath.Dir(tsvFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	// Write header if file doesn't exist
	needsHeader := false
	if _, err := os.Stat(tsvFile); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(tsvFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot open registry: %v\n", err)
		return
	}
	defer f.Close()

	if needsHeader {
		fmt.Fprintln(f, "build_id\ttimestamp\tstatus\tmin_rss\tmax_rss\tmean_rss\tp95_rss\tmin_cpu\tmax_cpu\tmean_cpu\tp95_cpu\tmin_load\tmax_load\tmean_load\tp95_load\tmin_disk_r\tmax_disk_r\tmean_disk_r\tp95_disk_r\tmin_disk_w\tmax_disk_w\tmean_disk_w\tp95_disk_w")
	}

	fmt.Fprintf(f, "%s\t%s\t%s\t%.0f\t%.0f\t%.1f\t%.0f\t%.1f\t%.1f\t%.1f\t%.1f\t%.2f\t%.2f\t%.2f\t%.2f\t%.0f\t%.0f\t%.1f\t%.0f\t%.0f\t%.0f\t%.1f\t%.0f\n",
		rec.BuildID, rec.Timestamp, rec.Status,
		rec.MinRSS, rec.MaxRSS, rec.MeanRSS, rec.P95RSS,
		rec.MinCPU, rec.MaxCPU, rec.MeanCPU, rec.P95CPU,
		rec.MinLoad, rec.MaxLoad, rec.MeanLoad, rec.P95Load,
		rec.MinDiskR, rec.MaxDiskR, rec.MeanDiskR, rec.P95DiskR,
		rec.MinDiskW, rec.MaxDiskW, rec.MeanDiskW, rec.P95DiskW)
}

func saveSummary(runDir string, rec *BuildRecord) {
	path := filepath.Join(runDir, summaryFile)
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------------------------
// TSV registry reader

func readRegistry() ([]*BuildRecord, error) {
	f, err := os.Open(tsvFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []*BuildRecord
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || lineNo == 1 {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < registryColCount {
			continue
		}
		rec := &BuildRecord{
			BuildID:   fields[0],
			Timestamp: fields[1],
			Status:    fields[2],
		}
		parseFloatField(fields[3], &rec.MinRSS)
		parseFloatField(fields[4], &rec.MaxRSS)
		parseFloatField(fields[5], &rec.MeanRSS)
		parseFloatField(fields[6], &rec.P95RSS)
		parseFloatField(fields[7], &rec.MinCPU)
		parseFloatField(fields[8], &rec.MaxCPU)
		parseFloatField(fields[9], &rec.MeanCPU)
		parseFloatField(fields[10], &rec.P95CPU)
		parseFloatField(fields[11], &rec.MinLoad)
		parseFloatField(fields[12], &rec.MaxLoad)
		parseFloatField(fields[13], &rec.MeanLoad)
		parseFloatField(fields[14], &rec.P95Load)
		parseFloatField(fields[15], &rec.MinDiskR)
		parseFloatField(fields[16], &rec.MaxDiskR)
		parseFloatField(fields[17], &rec.MeanDiskR)
		parseFloatField(fields[18], &rec.P95DiskR)
		parseFloatField(fields[19], &rec.MinDiskW)
		parseFloatField(fields[20], &rec.MaxDiskW)
		parseFloatField(fields[21], &rec.MeanDiskW)
		parseFloatField(fields[22], &rec.P95DiskW)
		records = append(records, rec)
	}
	return records, scanner.Err()
}

func parseFloatField(s string, target *float64) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err == nil {
		*target = v
	}
}

// ---------------------------------------------------------------------------
// Report generation — writes docs/Stats.md

func generateReport() error {
	initPaths()
	records, err := readRegistry()
	if err != nil {
		// No records yet -- create empty report
		records = nil
	}

	// Ensure docs/ directory exists
	os.MkdirAll(filepath.Join(projectRoot, "docs"), 0755)

	f, err := os.Create(filepath.Join(projectRoot, "docs", "Stats.md"))
	if err != nil {
		return fmt.Errorf("cannot create docs/Stats.md: %w", err)
	}
	defer f.Close()

	now := time.Now().UTC()

	fmt.Fprintf(f, `# Build Resource Stats

**Revision:** 1
**Last modified:** %s
**Source:** §11.4.24 Build-resource stats tracker

## Ever-values (across all tracked builds)

`, now.Format(time.RFC3339))

	if len(records) == 0 {
		fmt.Fprintln(f, "No builds tracked yet.")
	} else {
		// Compute ever-values
		ever := computeEverValues(records)
		fmt.Fprintf(f, `| Metric | Min | Max | Mean |
|--------|-----|-----|------|
| Memory (MB) | %.0f | %.0f | %.1f |
| CPU %% | %.1f | %.1f | %.1f |
| Load | %.2f | %.2f | %.2f |
| Disk Read (MB/sample) | %.0f | %.0f | %.1f |
| Disk Write (MB/sample) | %.0f | %.0f | %.1f |

`, ever.minRSS, ever.maxRSS, ever.meanRSS,
			ever.minCPU, ever.maxCPU, ever.meanCPU,
			ever.minLoad, ever.maxLoad, ever.meanLoad,
			ever.minDiskR, ever.maxDiskR, ever.meanDiskR,
			ever.minDiskW, ever.maxDiskW, ever.meanDiskW)
	}

	fmt.Fprintf(f, `## Per-build

| Build ID | Date | Status | Memory (MB) | CPU %% | Load | Disk R (MB) | Disk W (MB) |
|----------|------|--------|-------------|-------|------|-------------|-------------|
`)

	// Print most-recent-first
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		fmt.Fprintf(f, "| %s | %s | %s | %.0f/%.0f/%.1f | %.1f/%.1f/%.1f | %.2f | %.0f | %.0f |\n",
			r.BuildID, r.Timestamp, r.Status,
			r.MinRSS, r.MaxRSS, r.MeanRSS,
			r.MinCPU, r.MaxCPU, r.MeanCPU,
			r.MeanLoad,
			r.MeanDiskR, r.MeanDiskW)
	}

	fmt.Fprintf(f, `
---

*Generated by build-stats at %s*
`, now.Format(time.RFC3339))

	return nil
}

type everValues struct {
	minRSS, maxRSS, meanRSS       float64
	minCPU, maxCPU, meanCPU       float64
	minLoad, maxLoad, meanLoad    float64
	minDiskR, maxDiskR, meanDiskR float64
	minDiskW, maxDiskW, meanDiskW float64
}

func computeEverValues(records []*BuildRecord) everValues {
	var ev everValues
	var rssVals, cpuVals, loadVals, diskRVals, diskWVals []float64

	for _, r := range records {
		rssVals = append(rssVals, r.MeanRSS)
		cpuVals = append(cpuVals, r.MeanCPU)
		loadVals = append(loadVals, r.MeanLoad)
		diskRVals = append(diskRVals, r.MeanDiskR)
		diskWVals = append(diskWVals, r.MeanDiskW)
	}

	if len(rssVals) > 0 {
		ev.minRSS = minOf(rssVals)
		ev.maxRSS = maxOf(rssVals)
		ev.meanRSS = meanOf(rssVals)
	}
	if len(cpuVals) > 0 {
		ev.minCPU = minOf(cpuVals)
		ev.maxCPU = maxOf(cpuVals)
		ev.meanCPU = meanOf(cpuVals)
	}
	if len(loadVals) > 0 {
		ev.minLoad = minOf(loadVals)
		ev.maxLoad = maxOf(loadVals)
		ev.meanLoad = meanOf(loadVals)
	}
	if len(diskRVals) > 0 {
		ev.minDiskR = minOf(diskRVals)
		ev.maxDiskR = maxOf(diskRVals)
		ev.meanDiskR = meanOf(diskRVals)
	}
	if len(diskWVals) > 0 {
		ev.minDiskW = minOf(diskWVals)
		ev.maxDiskW = maxOf(diskWVals)
		ev.meanDiskW = meanOf(diskWVals)
	}
	return ev
}

// ---------------------------------------------------------------------------
// Subcommand handlers

func cmdStart() {
	initPaths()
	ts := time.Now().UTC().Format("20060102T150405Z")
	runDir := filepath.Join(dataDir, ts)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create run dir %s: %v\n", runDir, err)
		os.Exit(1)
	}

	// Fork daemon
	cmd := exec.Command(os.Args[0], "_daemon", runDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot start sampler: %v\n", err)
		os.Exit(1)
	}

	// Write PID
	pidPath := filepath.Join(runDir, pidFileName)
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d\n", cmd.Process.Pid)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot write PID file: %v\n", err)
	}

	// Write current_run symlink
	currentPath := filepath.Join(dataDir, runDirLink)
	os.Remove(currentPath)
	relDir, _ := filepath.Rel(dataDir, runDir)
	if relDir != "" {
		if err := os.Symlink(ts, currentPath); err != nil {
			// Symlink may fail on some OS; not critical
		}
	}

	fmt.Printf("Sampler started (PID %d) at %s\n", cmd.Process.Pid, runDir)
}

func cmdStop() {
	initPaths()
	// Find the run dir from the symlink
	currentPath := filepath.Join(dataDir, runDirLink)
	runDir := ""
	if link, err := os.Readlink(currentPath); err == nil {
		runDir = filepath.Join(dataDir, link)
	}

	if runDir == "" {
		// Fallback: find the most recent run directory
		entries, err := os.ReadDir(dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: no build-stats data: %v\n", err)
			os.Exit(1)
		}
		var latest string
		for _, e := range entries {
			if e.IsDir() && e.Name() != runDirLink && e.Name()[0] >= '2' {
				latest = e.Name()
			}
		}
		if latest == "" {
			fmt.Fprintln(os.Stderr, "ERROR: no build stats run to stop")
			os.Exit(1)
		}
		runDir = filepath.Join(dataDir, latest)
	}

	pidPath := filepath.Join(runDir, pidFileName)
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot read PID file: %v\n", err)
		os.Exit(1)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid PID: %v\n", err)
		os.Exit(1)
	}

	// Send SIGTERM
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot find process %d: %v\n", pid, err)
		os.Exit(1)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot signal sampler: %v\n", err)
		os.Exit(1)
	}

	// Wait for the process to exit
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Sampler stopped.")
	case <-time.After(5 * time.Second):
		fmt.Println("WARN: sampler did not exit cleanly, sending SIGKILL")
		proc.Kill()
		<-done
	}

	// Read summary and display
	summaryPath := filepath.Join(runDir, summaryFile)
	if data, err := os.ReadFile(summaryPath); err == nil {
		fmt.Printf("Build stats summary:\n%s\n", string(data))
	} else {
		fmt.Fprintf(os.Stderr, "WARN: no summary file: %v\n", err)
	}

	fmt.Printf("Stats recorded in %s\n", tsvFile)
}

func cmdReport() {
	initPaths()
	if err := generateReport(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: generating report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Report written to %s\n", filepath.Join(projectRoot, "docs", "Stats.md"))
}

func cmdServe(args []string) {
	initPaths()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: build-stats serve -- <command> [args...]")
		os.Exit(1)
	}

	cmdStart()

	buildCmd := exec.Command(args[0], args[1:]...)
	buildCmd.Stdin = os.Stdin
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	// Determine build status from exit code
	exitCode := 0
	if err := buildCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Update status based on exit code
	// Read the current_run link and update the summary later via compute
	cmdStop()

	// If build failed, rewrite the status in the registry
	if exitCode != 0 {
		// Read latest summary and mark as FAIL
		currentPath := filepath.Join(dataDir, runDirLink)
		if link, err := os.Readlink(currentPath); err == nil {
			runDir := filepath.Join(dataDir, link)
			summaryPath := filepath.Join(runDir, summaryFile)
			if data, err := os.ReadFile(summaryPath); err == nil {
				var rec BuildRecord
				if json.Unmarshal(data, &rec) == nil {
					rec.Status = "FAIL"
					saveSummary(runDir, &rec)
					// Re-append to correct the status
					// For now, just note it
					_ = rec
				}
			}
		}
	}

	cmdReport()
	os.Exit(exitCode)
}

// ---------------------------------------------------------------------------

func printUsage() {
	fmt.Fprintf(os.Stderr, `Build-resource stats tracker — §11.4.24

Usage:
  build-stats start              Start background sampler
  build-stats stop               Stop sampler, compute stats, append to TSV
  build-stats report             Generate docs/Stats.md from TSV
  build-stats serve -- <cmd...>  Wrap a command with resource tracking
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	initPaths()

	switch os.Args[1] {
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "report":
		cmdReport()
	case "serve":
		// Find the -- separator
		args := os.Args[2:]
		for i, a := range args {
			if a == "--" {
				cmdServe(args[i+1:])
				return
			}
		}
		// No -- separator: treat all remaining args as the command
		cmdServe(args)
	case "_daemon":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "ERROR: _daemon requires run directory")
			os.Exit(1)
		}
		runDaemon(os.Args[2])
	default:
		printUsage()
		os.Exit(1)
	}
}
