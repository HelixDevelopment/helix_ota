package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// TestListDevicesPaginationIsStable proves ListDevices' offset-cursor
// pagination visits every matching device exactly once when paged end-to-end.
//
// Root cause (fixed): ListDevices used to build its matched slice by ranging
// directly over the m.devices map. Go map iteration order is randomized per
// range statement (the language spec explicitly does not guarantee the same
// order "from one iteration to the next"), so two separate ListDevices calls
// against the SAME unchanged map can — and reliably do, for a map of any real
// size — visit entries in different orders. The offset-based cursor
// (start:end slicing) assumes a STABLE order between the call that minted a
// cursor and the call that consumes it; every OTHER paginated/list method in
// this file (ListReleases, ListAudit, ListGroups, ListProjects,
// ListFabricTargets) avoids this by ranging over a maintained insertion-order
// slice (relOrder/grpOrder/prjOrder/fabTgtOrder) instead of its map directly —
// ListDevices was the one exception, with no devOrder slice at all.
//
// Captured evidence before the fix (300 devices, page size 7): 43 pages,
// dupes=68-85, missing=104-125 (varies run to run because the reordering is
// randomized) — i.e. roughly a quarter of the fleet either double-counted or
// silently dropped from a full paginated sweep. After the fix (devOrder added,
// ListDevices/AllDevices range over it) the same sweep reproducibly returns
// dupes=0, missing=0 (asserted below; also observed identical across 3
// consecutive runs during the fix's verification).
func TestListDevicesPaginationIsStable(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	const n = 300
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("dev-%04d", i)
		if err := r.CreateDevice(ctx, Device{DeviceID: id, HardwareID: id + "-hw"}); err != nil {
			t.Fatalf("CreateDevice %s: %v", id, err)
		}
	}

	seen := make(map[string]int, n)
	cursor := ""
	pages := 0
	for {
		page, next, err := r.ListDevices(ctx, DeviceFilter{Limit: 7, Cursor: cursor})
		if err != nil {
			t.Fatalf("ListDevices: %v", err)
		}
		pages++
		if pages > n { // n/7 pages expected (~43); this is a generous runaway guard
			t.Fatalf("runaway pagination: still not exhausted after %d pages", pages)
		}
		for _, d := range page {
			seen[d.DeviceID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}

	var dupes, total int
	for id, count := range seen {
		total += count
		if count > 1 {
			dupes++
			t.Errorf("device %s returned %d times across the paginated sweep", id, count)
		}
	}
	missing := n - len(seen)
	if missing != 0 || dupes != 0 {
		t.Fatalf("pagination sweep over %d pages: unique=%d dupes=%d missing=%d (want unique=%d dupes=0 missing=0)",
			pages, len(seen), dupes, missing, n)
	}
	if total != n {
		t.Fatalf("total returned across all pages = %d, want %d", total, n)
	}
}

// TestListDevicesPaginationPartitionsFilteredSet is the same proof restricted
// to a filtered subset (the common real-world case: paging one os_type/model),
// smaller and run under -race by the package's race sweep.
func TestListDevicesPaginationPartitionsFilteredSet(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	const n = 120
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("android-%03d", i)
		if err := r.CreateDevice(ctx, Device{DeviceID: id, HardwareID: id + "-hw", Model: "OrangePi5Max"}); err != nil {
			t.Fatalf("CreateDevice %s: %v", id, err)
		}
	}
	// A handful of non-matching devices interleaved by insertion so the filter
	// actually has to skip entries mid-scan, not just take a clean prefix.
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("other-%03d", i)
		if err := r.CreateDevice(ctx, Device{DeviceID: id, HardwareID: id + "-hw", Model: "SomeOtherBoard"}); err != nil {
			t.Fatalf("CreateDevice %s: %v", id, err)
		}
	}

	seen := make(map[string]int, n)
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > n {
			t.Fatalf("runaway pagination after %d pages", pages)
		}
		page, next, err := r.ListDevices(ctx, DeviceFilter{TargetModel: "OrangePi5Max", Limit: 5, Cursor: cursor})
		if err != nil {
			t.Fatalf("ListDevices: %v", err)
		}
		for _, d := range page {
			if d.Model != "OrangePi5Max" {
				t.Fatalf("filter leaked non-matching device %s (model=%s)", d.DeviceID, d.Model)
			}
			seen[d.DeviceID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	for id, count := range seen {
		if count != 1 {
			t.Fatalf("device %s returned %d times, want exactly 1", id, count)
		}
	}
	if len(seen) != n {
		t.Fatalf("filtered pagination sweep returned %d unique devices, want %d", len(seen), n)
	}
}

// TestMemoryDeviceConcurrentCreateAndListRace proves the devOrder slice this
// fix introduced is safe under concurrent CreateDevice calls racing concurrent
// ListDevices/AllDevices/GetDevice readers (run with -race; ≥50 goroutines per
// §11.4.85 stress-test concurrent-contention floor). devOrder is appended under
// the same m.mu.Lock() as the devices/devByHW map writes, so this must be
// race-free and every created device must appear in AllDevices exactly once
// once all writers finish.
func TestMemoryDeviceConcurrentCreateAndListRace(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRepository()

	const writers = 64
	const readersPerWriter = 2

	var wg sync.WaitGroup
	wg.Add(writers * (1 + readersPerWriter))
	for i := 0; i < writers; i++ {
		i := i
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("race-dev-%03d", i)
			if err := r.CreateDevice(ctx, Device{
				DeviceID: id, HardwareID: id + "-hw",
				Metadata: map[string]string{"idx": fmt.Sprintf("%d", i)},
			}); err != nil {
				t.Errorf("CreateDevice %s: %v", id, err)
			}
		}()
		for j := 0; j < readersPerWriter; j++ {
			go func() {
				defer wg.Done()
				_, _, _ = r.ListDevices(ctx, DeviceFilter{Limit: 10})
				_ = r.AllDevices(ctx)
				_, _ = r.GetDevice(ctx, fmt.Sprintf("race-dev-%03d", i))
			}()
		}
	}
	wg.Wait()

	all := r.AllDevices(ctx)
	if len(all) != writers {
		t.Fatalf("AllDevices after concurrent creates: got %d, want %d", len(all), writers)
	}
	seen := make(map[string]bool, writers)
	for _, d := range all {
		if seen[d.DeviceID] {
			t.Fatalf("duplicate device %s in AllDevices after concurrent creates", d.DeviceID)
		}
		seen[d.DeviceID] = true
	}
	for i := 0; i < writers; i++ {
		id := fmt.Sprintf("race-dev-%03d", i)
		if !seen[id] {
			t.Fatalf("device %s missing from AllDevices after concurrent creates", id)
		}
	}
}
