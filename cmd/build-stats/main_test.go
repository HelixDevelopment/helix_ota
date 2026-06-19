package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Stats computation tests

func TestMinOf(t *testing.T) {
	if got := minOf([]float64{3, 1, 4, 1, 5}); got != 1 {
		t.Errorf("minOf = %v, want 1", got)
	}
	if got := minOf([]float64{42}); got != 42 {
		t.Errorf("minOf single = %v, want 42", got)
	}
	if got := minOf(nil); got != 0 {
		t.Errorf("minOf nil = %v, want 0", got)
	}
}

func TestMaxOf(t *testing.T) {
	if got := maxOf([]float64{3, 1, 4, 1, 5}); got != 5 {
		t.Errorf("maxOf = %v, want 5", got)
	}
	if got := maxOf([]float64{42}); got != 42 {
		t.Errorf("maxOf single = %v, want 42", got)
	}
	if got := maxOf(nil); got != 0 {
		t.Errorf("maxOf nil = %v, want 0", got)
	}
}

func TestMeanOf(t *testing.T) {
	if got := meanOf([]float64{2, 4, 6}); got != 4.0 {
		t.Errorf("meanOf = %v, want 4.0", got)
	}
	if got := meanOf([]float64{10}); got != 10.0 {
		t.Errorf("meanOf single = %v, want 10.0", got)
	}
	if got := meanOf(nil); got != 0 {
		t.Errorf("meanOf nil = %v, want 0", got)
	}
}

func TestP95Of(t *testing.T) {
	// 20 values: p95 at index ceil(0.95*20)-1 = ceil(19)-1 = 19-1 = 18
	vals := make([]float64, 20)
	for i := range vals {
		vals[i] = float64(i + 1) // 1..20
	}
	if got := p95Of(vals); got != 19 {
		t.Errorf("p95Of(1..20) = %v, want 19", got)
	}

	// Single value
	if got := p95Of([]float64{42}); got != 42 {
		t.Errorf("p95Of single = %v, want 42", got)
	}

	// Empty slice
	if got := p95Of(nil); got != 0 {
		t.Errorf("p95Of nil = %v, want 0", got)
	}
}

func TestP95I(t *testing.T) {
	vals := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	if got := p95I(vals); got != 19 {
		t.Errorf("p95I(1..20) = %v, want 19", got)
	}
}

func TestMeanI(t *testing.T) {
	m := meanI([]int64{2, 4, 6})
	if math.Abs(m-4.0) > 0.001 {
		t.Errorf("meanI = %v, want 4.0", m)
	}
}

// ---------------------------------------------------------------------------
// Sample and stats computation integration

func TestComputeStats(t *testing.T) {
	samples := []*Sample{
		{Timestamp: 1000, RSSMB: 8000, CPU: 120.5, Load1: 3.5, Load5: 3.0, Load15: 2.5, DiskR: 1000, DiskW: 500},
		{Timestamp: 6000, RSSMB: 8200, CPU: 150.3, Load1: 4.2, Load5: 3.5, Load15: 2.8, DiskR: 1200, DiskW: 650},
		{Timestamp: 11000, RSSMB: 8100, CPU: 130.1, Load1: 3.8, Load5: 3.2, Load15: 2.6, DiskR: 1350, DiskW: 720},
	}

	rec := computeStats(samples, "test-build", "2026-06-19T00:00:00Z")

	if rec.MinRSS != 8000 {
		t.Errorf("MinRSS = %v, want 8000", rec.MinRSS)
	}
	if rec.MaxRSS != 8200 {
		t.Errorf("MaxRSS = %v, want 8200", rec.MaxRSS)
	}
	if rec.MeanRSS != 8100 {
		t.Errorf("MeanRSS = %v, want 8100", rec.MeanRSS)
	}

	if rec.MinCPU != 120.5 {
		t.Errorf("MinCPU = %v, want 120.5", rec.MinCPU)
	}
	if rec.MaxCPU != 150.3 {
		t.Errorf("MaxCPU = %v, want 150.3", rec.MaxCPU)
	}

	if rec.MinLoad != 3.5 {
		t.Errorf("MinLoad = %v, want 3.5", rec.MinLoad)
	}
	if rec.MaxLoad != 4.2 {
		t.Errorf("MaxLoad = %v, want 4.2", rec.MaxLoad)
	}

	// Disk deltas: sample2 diskR - sample1 diskR = 200, sample3 diskR - sample2 diskR = 150
	if rec.MinDiskR != 150 {
		t.Errorf("MinDiskR = %v, want 150", rec.MinDiskR)
	}
	if rec.MaxDiskR != 200 {
		t.Errorf("MaxDiskR = %v, want 200", rec.MaxDiskR)
	}
	if rec.MeanDiskR != 175 {
		t.Errorf("MeanDiskR = %v, want 175", rec.MeanDiskR)
	}
}

// ---------------------------------------------------------------------------
// TSV I/O tests

func TestAppendAndReadRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	origTsv := tsvFile
	tsvFile = filepath.Join(tmpDir, "build_stats.tsv")
	defer func() { tsvFile = origTsv }()

	rec1 := &BuildRecord{
		BuildID:   "abc123",
		Timestamp: "2026-06-19T00:00:00Z",
		Status:    "SUCCESS",
		MinRSS:    8000, MaxRSS: 8200, MeanRSS: 8100, P95RSS: 8150,
		MinCPU: 100, MaxCPU: 200, MeanCPU: 150, P95CPU: 190,
		MinLoad: 2.5, MaxLoad: 4.0, MeanLoad: 3.2, P95Load: 3.8,
		MinDiskR: 100, MaxDiskR: 300, MeanDiskR: 200, P95DiskR: 290,
		MinDiskW: 50, MaxDiskW: 150, MeanDiskW: 100, P95DiskW: 140,
	}

	appendRecord(rec1)

	// Read back
	records, err := readRegistry()
	if err != nil {
		t.Fatalf("readRegistry: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	r := records[0]
	if r.BuildID != "abc123" {
		t.Errorf("BuildID = %q, want %q", r.BuildID, "abc123")
	}
	if r.Status != "SUCCESS" {
		t.Errorf("Status = %q, want %q", r.Status, "SUCCESS")
	}
	if r.MeanRSS != 8100 {
		t.Errorf("MeanRSS = %v, want 8100", r.MeanRSS)
	}
}

func TestReadEmptyRegistry(t *testing.T) {
	origTsv := tsvFile
	tsvFile = filepath.Join(t.TempDir(), "nonexistent.tsv")
	defer func() { tsvFile = origTsv }()

	records, err := readRegistry()
	if err == nil {
		if len(records) != 0 {
			t.Errorf("expected empty, got %d records", len(records))
		}
	}
	// err != nil is acceptable (file not found)
}

// ---------------------------------------------------------------------------
// Ever-values computation test

func TestComputeEverValues(t *testing.T) {
	records := []*BuildRecord{
		{
			MeanRSS: 1000, MeanCPU: 50, MeanLoad: 2.0, MeanDiskR: 100, MeanDiskW: 50,
		},
		{
			MeanRSS: 2000, MeanCPU: 80, MeanLoad: 3.0, MeanDiskR: 200, MeanDiskW: 100,
		},
		{
			MeanRSS: 1500, MeanCPU: 65, MeanLoad: 2.5, MeanDiskR: 150, MeanDiskW: 75,
		},
	}

	ev := computeEverValues(records)

	if ev.minRSS != 1000 {
		t.Errorf("minRSS = %v, want 1000", ev.minRSS)
	}
	if ev.maxRSS != 2000 {
		t.Errorf("maxRSS = %v, want 2000", ev.maxRSS)
	}
	if ev.minCPU != 50 {
		t.Errorf("minCPU = %v, want 50", ev.minCPU)
	}
	if ev.maxCPU != 80 {
		t.Errorf("maxCPU = %v, want 80", ev.maxCPU)
	}
	if ev.meanLoad != 2.5 {
		t.Errorf("meanLoad = %v, want 2.5", ev.meanLoad)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests

func TestExtractInt(t *testing.T) {
	if v := extractInt("Pages free:    12345."); v != 12345 {
		t.Errorf("extractInt = %v, want 12345", v)
	}
	if v := extractInt("pageins: 67890"); v != 67890 {
		t.Errorf("extractInt = %v, want 67890", v)
	}
	if v := extractInt("no number here"); v != 0 {
		t.Errorf("extractInt = %v, want 0", v)
	}
}

func TestExtractKB(t *testing.T) {
	if v := extractKB("MemTotal:       8192000 kB"); v != 8192000 {
		t.Errorf("extractKB = %v, want 8192000", v)
	}
}

func TestIsMainDevice(t *testing.T) {
	if !isMainDevice("sda") {
		t.Error("isMainDevice(sda) should be true")
	}
	if !isMainDevice("nvme0n1") {
		t.Error("isMainDevice(nvme0n1) should be true")
	}
	if !isMainDevice("vda") {
		t.Error("isMainDevice(vda) should be true")
	}
	if isMainDevice("sda1") {
		t.Error("isMainDevice(sda1) should be false")
	}
	if isMainDevice("nvme0n1p1") {
		t.Error("isMainDevice(nvme0n1p1) should be false")
	}
}

// ---------------------------------------------------------------------------
// Sample read/write round-trip

func TestSampleRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	samplePath := filepath.Join(tmpDir, "samples.tsv")

	// Write header + data
	if err := os.WriteFile(samplePath, []byte("ts\trss_mb\tcpu_pct\tload1\tload5\tload15\tdisk_r_mb\tdisk_w_mb\n1000\t8000\t120.5\t3.5\t3.0\t2.5\t1000\t500\n6000\t8200\t150.3\t4.2\t3.5\t2.8\t1200\t650\n"), 0644); err != nil {
		t.Fatal(err)
	}

	samples := readSamples(samplePath)
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	if samples[0].RSSMB != 8000 {
		t.Errorf("sample[0].RSSMB = %v, want 8000", samples[0].RSSMB)
	}
	if samples[0].CPU != 120.5 {
		t.Errorf("sample[0].CPU = %v, want 120.5", samples[0].CPU)
	}
	if samples[1].RSSMB != 8200 {
		t.Errorf("sample[1].RSSMB = %v, want 8200", samples[1].RSSMB)
	}
	if samples[1].DiskR != 1200 {
		t.Errorf("sample[1].DiskR = %v, want 1200", samples[1].DiskR)
	}
}
