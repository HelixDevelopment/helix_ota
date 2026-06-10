// Command ota-device-emu is a thin CLI around the deviceemu.Device — a faithful
// emulator of an RK3588 / Orange Pi 5 Max OTA client speaking the real Helix OTA
// HTTP control-plane protocol. It registers a device, polls for an update, and
// (when offered) reports the full apply lifecycle, then prints a JSON outcome.
//
// It is a Tier-1 testing tool: it substitutes for a physical device, NOT for the
// server. Flashing is the only emulated act (see the deviceemu package doc); the
// login, registration, update-check, and telemetry are all real HTTP calls.
//
// Usage:
//
//	ota-device-emu -base http://127.0.0.1:8080/api/v1 \
//	  -admin-user admin@helix.example -admin-pass "$HELIX_ADMIN_PASSWORD" \
//	  -hardware-id rk3588-AABBCCDD -model OrangePi5Max -os android \
//	  -current-version 1.0.0 -once
//
// With -loop it polls on -interval until interrupted (Ctrl-C). With -deployment
// it attaches the operator-supplied deployment id to telemetry (see the
// deviceemu protocol-gap note: the id cannot be derived from the update offer).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HelixDevelopment/helix_ota/server/internal/deviceemu"
)

func main() {
	var (
		base       = flag.String("base", "http://127.0.0.1:8080/api/v1", "server base URL including API base path")
		adminUser  = flag.String("admin-user", "", "operator username (or set -operator-token)")
		adminPass  = flag.String("admin-pass", "", "operator password (or set -operator-token)")
		opToken    = flag.String("operator-token", "", "pre-obtained operator bearer token (skips login)")
		hardwareID = flag.String("hardware-id", "", "device hardware id (required)")
		model      = flag.String("model", "OrangePi5Max", "device model")
		osType     = flag.String("os", "android", "device OS type (android|linux|windows|other)")
		current    = flag.String("current-version", "", "device current firmware version")
		group      = flag.String("group", "", "deployment target group")
		deployment = flag.String("deployment", "", "operator-supplied deployment id for telemetry")
		once       = flag.Bool("once", true, "run a single check->apply cycle and exit")
		loop       = flag.Bool("loop", false, "poll on -interval until interrupted (overrides -once)")
		interval   = flag.Duration("interval", 15*time.Minute, "poll interval in -loop mode")
		timeout    = flag.Duration("timeout", 60*time.Second, "per-cycle context timeout")
	)
	flag.Parse()

	dev, err := deviceemu.New(deviceemu.Config{
		BaseURL:        *base,
		AdminUser:      *adminUser,
		AdminPass:      *adminPass,
		OperatorToken:  *opToken,
		HardwareID:     *hardwareID,
		Model:          *model,
		OSType:         *osType,
		CurrentVersion: *current,
		Group:          *group,
		DeploymentID:   *deployment,
	})
	if err != nil {
		fatal(err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if *loop {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err := dev.RunLoop(ctx, *interval,
			func(o deviceemu.Outcome) { _ = enc.Encode(o) },
			func(e error) { fmt.Fprintf(os.Stderr, "ota-device-emu: cycle error: %v\n", e) },
		)
		if err != nil && ctx.Err() == nil {
			fatal(err)
		}
		return
	}

	// Single cycle (default). -once is the explicit form.
	_ = once
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	out, runErr := dev.RunOnce(ctx)
	if runErr != nil {
		// Still emit whatever partial outcome we have for diagnosis, then fail.
		_ = enc.Encode(out)
		fatal(runErr)
	}
	if err := enc.Encode(out); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "ota-device-emu: %v\n", err)
	os.Exit(1)
}
