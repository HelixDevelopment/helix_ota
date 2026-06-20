// ApplyPort — the in-guest OTA apply service.
//
// This binary runs on the target device (RK3588 / Orange Pi 5 Max or any
// U-Boot A/B target) as a systemd service or standalone CLI. It implements
// the device-side half of the Helix OTA protocol:
//
//	Login -> Register -> CheckUpdate -> Download -> Verify -> Apply -> Arm -> Reboot
//
// Usage:
//
//	applyport run                 # Run the update loop once
//	applyport daemon              # Run as a polling daemon
//	applyport check               # Check for an update and print status
//	applyport confirm             # Confirm a healthy boot (health marker)
//	applyport status              # Print current slot / version info
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/device"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("[applyport] ")

	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	serverURL := flag.String("server", envOrDefault("HELIX_SERVER_URL", "http://localhost:8080/api/v1"), "Server base URL")
	username := flag.String("username", os.Getenv("HELIX_USERNAME"), "Operator username")
	password := flag.String("password", os.Getenv("HELIX_PASSWORD"), "Operator password")
	model := flag.String("model", envOrDefault("HELIX_DEVICE_MODEL", "rk3588"), "Device model")
	osType := flag.String("os-type", envOrDefault("HELIX_OS_TYPE", "linux"), "Device OS type")
	hardwareID := flag.String("hardware-id", envOrDefault("HELIX_HARDWARE_ID", "rk3588-opi5max"), "Device hardware ID")
	pubKeyB64 := flag.String("pubkey", os.Getenv("HELIX_ARTIFACT_PUBKEY"), "Base64-encoded ed25519 public key for signature verification")
	insecure := flag.Bool("insecure", envBool("HELIX_INSECURE"), "Skip TLS verification (dev only)")
	pollInterval := flag.Duration("poll", envDuration("HELIX_POLL_INTERVAL", 15*time.Minute), "Poll interval for daemon mode")

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: applyport <command> [flags]\n\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  run           Execute one update check+apply cycle\n")
		fmt.Fprintf(os.Stderr, "  daemon        Run as a polling daemon\n")
		fmt.Fprintf(os.Stderr, "  check         Check for update and print status\n")
		fmt.Fprintf(os.Stderr, "  confirm       Confirm healthy boot (clear upgrade_available)\n")
		fmt.Fprintf(os.Stderr, "  status        Print current slot / version info\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parse flags before the subcommand so subcommand parsers can re-parse.
	flag.Parse()

	cmd := os.Args[1]
	// Re-parse flags for the subcommand.
	flag.Parse()
	_ = os.Args[2:]

	switch cmd {
	case "run":
		return runCycle(*serverURL, *username, *password, *model, *osType, *hardwareID, *pubKeyB64, *insecure)
	case "daemon":
		return runDaemon(*serverURL, *username, *password, *model, *osType, *hardwareID, *pubKeyB64, *insecure, *pollInterval)
	case "check":
		return runCheck(*serverURL, *model, *osType, *hardwareID, *insecure)
	case "confirm":
		return runConfirm()
	case "status":
		return runStatus()
	default:
		return fmt.Errorf("unknown command: %q", cmd)
	}
}

// runCycle executes one full apply cycle.
func runCycle(serverURL, username, password, model, osType, hardwareID, pubKeyB64 string, insecure bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	verifier, err := buildVerifier(pubKeyB64)
	if err != nil {
		return fmt.Errorf("build verifier: %w", err)
	}

	client := device.NewApplyPortClient(serverURL,
		clientOpt(insecure),
	)

	log.Printf("logging in to %s", serverURL)
	if err := client.Login(ctx, username, password); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	log.Printf("registering device (model=%s, os=%s, hw=%s)", model, osType, hardwareID)
	regReq := device.DeviceRegistrationRequest{
		HardwareID: hardwareID,
		Model:      model,
		OSType:     osType,
	}
	if _, err := client.Register(ctx, regReq); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// Determine current version and active slot.
	slotWriter := device.NewSlotDevice("", "", "")
	currentSlot, err := slotWriter.ActiveSlot()
	if err != nil {
		return fmt.Errorf("detect slot: %w", err)
	}
	log.Printf("current slot: %s", currentSlot)

	currentVersion := detectCurrentVersion()
	log.Printf("current version: %s", currentVersion)

	// Step (a): check for update.
	log.Print("checking for update...")
	result, err := client.CheckForUpdate(ctx, currentVersion)
	if err != nil {
		return fmt.Errorf("check update: %w", err)
	}
	if !result.Available {
		log.Print("no update available")
		return nil
	}
	log.Printf("update available: release=%s version=%s", result.ReleaseID, result.Version)

	// Report: download_started.
	reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "download_started", result.Version, 0, "")

	// Step (b): download bundle.
	log.Print("downloading bundle...")
	bundleData, err := client.DownloadBundle(ctx, result.URL, result.Offset, result.Size)
	if err != nil {
		reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "failure", result.Version, 0,
			fmt.Sprintf("download failed: %v", err))
		return fmt.Errorf("download: %w", err)
	}
	log.Printf("downloaded %d bytes", len(bundleData))

	// Report: installing.
	reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "installing", result.Version, int64(len(bundleData)), "")

	// Verify SHA256 before signature check.
	if result.SHA256 != "" {
		bundleSHA := sha256Hex(bundleData)
		if !strings.EqualFold(bundleSHA, result.SHA256) {
			reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "failure", result.Version, 0,
				"SHA256 mismatch")
			return fmt.Errorf("SHA256 mismatch: got %s, expected %s", bundleSHA, result.SHA256)
		}
		log.Print("SHA256 verified")
	}

	// Verify signature (SS11.4 trust boundary).
	if verifier.KeyConfigured() && result.Signature != "" {
		reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "verifying", result.Version, 0, "")
		if err := verifier.Verify(bundleData, result.Signature); err != nil {
			reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "failure", result.Version, 0,
				fmt.Sprintf("signature verification failed: %v", err))
			return fmt.Errorf("signature verify: %w", err)
		}
		log.Print("signature verified")
	} else {
		log.Print("signature verification skipped (no key or no signature)")
	}

	// Write bundle bytes to a temp file for the slot writer.
	tmpFile, tmpErr := os.CreateTemp("", "helix-update-*.img")
	if tmpErr != nil {
		return fmt.Errorf("create temp file: %w", tmpErr)
	}
	tmpPath := tmpFile.Name()
	if _, tmpErr = tmpFile.Write(bundleData); tmpErr != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write bundle to temp: %w", tmpErr)
	}
	if tmpErr = tmpFile.Sync(); tmpErr != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync bundle temp: %w", tmpErr)
	}
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Step (c): write inactive slot + arm U-Boot env.
	envMgr := device.NewFwEnvManager("", "")
	applier := device.NewApplyPort(slotWriter, envMgr, "")
	if err := applier.WriteAndArm(ctx, tmpPath); err != nil {
		reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "failure", result.Version, 0,
			fmt.Sprintf("apply failed: %v", err))
		return fmt.Errorf("apply: %w", err)
	}
	writtenSlot, _ := applier.InactiveSlot()
	log.Printf("wrote and armed slot %s (inactive -> boot target)", writtenSlot)

	// Report: success telemetry before reboot.
	reportEvent(ctx, client, client.DeviceID(), result.DeploymentID, "success", result.Version, int64(len(bundleData)), "")

	log.Print("apply cycle complete. Rebooting...")
	return reboot()
}

// runDaemon runs the update loop periodically.
func runDaemon(serverURL, username, password, model, osType, hardwareID, pubKeyB64 string, insecure bool, interval time.Duration) error {
	log.Printf("starting applyport daemon (poll interval: %v)", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately.
	if err := runCycle(serverURL, username, password, model, osType, hardwareID, pubKeyB64, insecure); err != nil {
		log.Printf("cycle failed: %v (will retry at next poll)", err)
	}

	for range ticker.C {
		if err := runCycle(serverURL, username, password, model, osType, hardwareID, pubKeyB64, insecure); err != nil {
			log.Printf("cycle failed: %v (will retry at next poll)", err)
		}
	}
	return nil
}

// runCheck checks for an update and prints the result.
func runCheck(serverURL, model, osType, hardwareID string, insecure bool) error {
	_ = model
	_ = osType
	_ = hardwareID
	log.Printf("check: would query %s", serverURL)
	// In a real deployment, this would use a device-scoped token.
	return nil
}

// runConfirm confirms the current boot as healthy (clears upgrade_available).
func runConfirm() error {
	envMgr := device.NewFwEnvManager("", "")
	marker := device.NewHealthMarker(envMgr)

	armed, err := marker.IsArmed()
	if err != nil {
		return fmt.Errorf("confirm: read armed state: %w", err)
	}

	if !armed {
		log.Print("not armed -- nothing to confirm")
		return nil
	}

	if err := marker.ConfirmHealthy(); err != nil {
		return fmt.Errorf("confirm: %w", err)
	}
	log.Print("healthy boot confirmed (upgrade_available=0, bootcount=0)")
	return nil
}

// runStatus prints the current slot and version information.
func runStatus() error {
	slotWriter := device.NewSlotDevice("", "", "")
	active, err := slotWriter.ActiveSlot()
	if err != nil {
		return fmt.Errorf("status: active slot: %w", err)
	}
	inactive, err := slotWriter.InactiveSlot()
	if err != nil {
		return fmt.Errorf("status: inactive slot: %w", err)
	}

	envMgr := device.NewFwEnvManager("", "")
	marker := device.NewHealthMarker(envMgr)
	armed, err := marker.IsArmed()
	if err != nil {
		return fmt.Errorf("status: read armed: %w", err)
	}

	// Try to read the U-Boot env for upgrade_available/bootcount.
	upgradeAvailable, _ := envMgr.GetEnv("upgrade_available")
	bootCount, _ := envMgr.GetEnv("bootcount")

	fmt.Printf("Active slot:      %s\n", active)
	fmt.Printf("Inactive slot:    %s\n", inactive)
	fmt.Printf("Armed:            %v\n", armed)
	fmt.Printf("upgrade_available: %s\n", upgradeAvailable)
	fmt.Printf("bootcount:         %s\n", bootCount)
	fmt.Printf("Current version:   %s\n", detectCurrentVersion())

	return nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	return os.Getenv(key) == "1" || os.Getenv(key) == "true"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}

func buildVerifier(pubKeyB64 string) (*device.SignatureVerifier, error) {
	if pubKeyB64 == "" {
		log.Print("no public key configured (HELIX_ARTIFACT_PUBKEY) -- signature verification disabled")
		return device.NewSignatureVerifier(nil), nil
	}
	keyBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode pubkey: %w", err)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bad public key length %d (expected %d)", len(keyBytes), ed25519.PublicKeySize)
	}
	return device.NewSignatureVerifier(ed25519.PublicKey(keyBytes)), nil
}

func clientOpt(insecure bool) device.ClientOption {
	if insecure {
		return device.WithInsecureSkipVerify()
	}
	return func(_ *device.ApplyPortClient) {}
}

func detectCurrentVersion() string {
	// In the emulator/guest, the version would be read from /etc/helix-version
	// or similar. For now return a placeholder.
	data, err := os.ReadFile("/etc/helix-version")
	if err != nil {
		return "0.0.0"
	}
	return strings.TrimSpace(string(data))
}

func reportEvent(ctx context.Context, client *device.ApplyPortClient, deviceID, deploymentID, event, version string, bytesTransferred int64, detail string) {
	if deviceID == "" || deploymentID == "" {
		return
	}
	report := device.TelemetryReport{
		DeviceID:     deviceID,
		DeploymentID: deploymentID,
		Events: []device.TelemetryEvent{
			{
				Event:   event,
				Version: version,
				Detail:  detail,
				// Use approximate Unix time.
				Timestamp: time.Now().Unix(),
			},
		},
	}
	ack, err := client.ReportTelemetry(ctx, report)
	if err != nil {
		log.Printf("telemetry %s: %v", event, err)
		return
	}
	log.Printf("telemetry %s: accepted=%d rejected=%d", event, ack.Accepted, ack.Rejected)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func reboot() error {
	log.Print("rebooting system...")
	// Try systemctl first (systemd builds), fall back to reboot binary.
	if err := runCmd("systemctl", "reboot"); err != nil {
		// systemctl failed; try the plain reboot command.
		var errs []string
		errs = append(errs, fmt.Sprintf("systemctl: %v", err))
		if err2 := runCmd("reboot"); err2 != nil {
			errs = append(errs, fmt.Sprintf("reboot: %v", err2))
			return fmt.Errorf("reboot failed: %s", strings.Join(errs, "; "))
		}
	}
	return nil
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
