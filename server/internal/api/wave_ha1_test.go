package api

import (
	"context"
	"fmt"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// ha1NoAllDevicesRepo embeds the store.Repository INTERFACE, so it exposes
// ListDevices (a required method) but NOT AllDevices — reproducing the
// production pgx backend, which does not implement the optional
// deviceLister/AllDevices capability.
type ha1NoAllDevicesRepo struct{ store.Repository }

// TestMatchingDevices_HA1_EnumeratesViaListDevices is the permanent HA-1
// regression guard (§11.4.115 GREEN polarity). matchingDevices previously
// type-asserted the OPTIONAL deviceLister (AllDevices) and returned an EMPTY
// set on any backend lacking it (i.e. production Postgres), so
// handleCreateDeployment reported 201 Created targeting ZERO devices
// (§11.4.108). It now enumerates via the REQUIRED ListDevices seam, which both
// backends implement.
//
// Anti-tautology anchor: re-injecting the original `if _, ok :=
// s.repo.(deviceLister); !ok { return nil, nil }` guard at the top of
// matchingDevices reproduces the defect — on a repo WITHOUT AllDevices it
// returns 0 devices → RED; without the guard (the fix) → GREEN.
func TestMatchingDevices_HA1_EnumeratesViaListDevices(t *testing.T) {
	inner := store.NewMemoryRepository()
	repo := &ha1NoAllDevicesRepo{Repository: inner}

	// Mirror production Postgres: the optional AllDevices capability is absent.
	var r store.Repository = repo
	if _, ok := r.(deviceLister); ok {
		t.Fatal("setup: ha1NoAllDevicesRepo must NOT implement deviceLister (to mirror the pgx backend)")
	}

	s := &Server{repo: repo}
	ctx := context.Background()
	rel := store.Release{OSType: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max", Version: "1.0.0"}

	for i := 0; i < 3; i++ {
		if err := repo.CreateDevice(ctx, store.Device{
			DeviceID: fmt.Sprintf("ha1-d%d", i), HardwareID: fmt.Sprintf("ha1-hw%d", i), OSType: otaprotocol.OSAndroid, Model: "OrangePi5Max",
		}); err != nil {
			t.Fatalf("create device: %v", err)
		}
	}
	// A non-matching device (different model) must be excluded.
	if err := repo.CreateDevice(ctx, store.Device{
		DeviceID: "ha1-other", HardwareID: "ha1-hw-other", OSType: otaprotocol.OSAndroid, Model: "RPi4",
	}); err != nil {
		t.Fatalf("create other device: %v", err)
	}

	got, err := s.matchingDevices(ctx, rel, "")
	if err != nil {
		t.Fatalf("matchingDevices returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("HA-1: matchingDevices must enumerate via ListDevices on a repo WITHOUT AllDevices (like Postgres); want 3 matching, got %d", len(got))
	}
}
