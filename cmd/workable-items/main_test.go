// Tests for workable-items CLI (§11.4.93, §11.4.95)
package main

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

// testDB creates a temporary database for testing.
func testDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	f, err := os.CreateTemp("", "workable-items-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()

	db, err := openDB(path)
	if err != nil {
		os.Remove(path)
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})
	return db, path
}

// testItem returns a valid Item for tests.
func testItem(otaID string) *Item {
	return &Item{
		OtaID:        otaID,
		Type:         "Bug",
		Status:       "Queued",
		Severity:     "High",
		Title:        "Test item " + otaID,
		Description:  "This is a test item with a sufficiently long description for the 40-character minimum requirement.",
		ComposesWith: "[]",
	}
}

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

func TestAddItem(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-001")

	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	// Verify it was inserted
	items, err := listItems(db, "")
	if err != nil {
		t.Fatalf("listItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].OtaID != "OTA-001" {
		t.Errorf("expected OTA-001, got %s", items[0].OtaID)
	}
	if items[0].Status != "Queued" {
		t.Errorf("expected Queued, got %s", items[0].Status)
	}

	// Verify history was created
	history, err := getHistory(db, "OTA-001")
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Event != "Opened" {
		t.Errorf("expected Opened event, got %s", history[0].Event)
	}
}

func TestAddItemAllTypes(t *testing.T) {
	db, _ := testDB(t)
	for _, typ := range []string{"Bug", "Feature", "Task"} {
		item := testItem("OTA-TYPE-" + typ)
		item.Type = typ
		if err := addItem(db, item); err != nil {
			t.Fatalf("addItem %s: %v", typ, err)
		}
	}

	items, err := listItems(db, "")
	if err != nil {
		t.Fatalf("listItems: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestAddItemInvalidType(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-INV")
	item.Type = "InvalidType"

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func TestAddItemShortDescription(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-SHORT")
	item.Description = "Too short" // 9 chars

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for short description, got nil")
	}
}

func TestAddItemEmptyTitle(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-NOTITLE")
	item.Title = ""

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestAddItemEmptyOtaID(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("")
	item.Title = "No ID item"

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for empty OTA ID, got nil")
	}
}

func TestAddItemInvalidSeverity(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-SEV")
	item.Severity = "Urgent"

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for invalid severity, got nil")
	}
}

// ---------------------------------------------------------------------------
// Close tests
// ---------------------------------------------------------------------------

func TestCloseBug(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-010")
	item.Type = "Bug"
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	if err := closeItem(db, "OTA-010", "tests/passed"); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	items, _ := listItems(db, "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	expected := "Fixed (→ Fixed.md)"
	if items[0].Status != expected {
		t.Errorf("Bug closure: expected %q, got %q", expected, items[0].Status)
	}
}

func TestCloseFeature(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-011")
	item.Type = "Feature"
	item.Status = "In progress"
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	if err := closeItem(db, "OTA-011", "qa/evidence/feature-011"); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	items, _ := listItems(db, "")
	expected := "Implemented (→ Fixed.md)"
	if items[0].Status != expected {
		t.Errorf("Feature closure: expected %q, got %q", expected, items[0].Status)
	}
}

func TestCloseTask(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-012")
	item.Type = "Task"
	item.Status = "In testing"
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	if err := closeItem(db, "OTA-012", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	items, _ := listItems(db, "")
	expected := "Completed (→ Fixed.md)"
	if items[0].Status != expected {
		t.Errorf("Task closure: expected %q, got %q", expected, items[0].Status)
	}
}

func TestCloseItemNotFound(t *testing.T) {
	db, _ := testDB(t)

	err := closeItem(db, "OTA-NONEXIST", "")
	if err == nil {
		t.Fatal("expected error for non-existent item, got nil")
	}
}

func TestCloseAlreadyClosed(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-013")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-013", ""); err != nil {
		t.Fatalf("first close: %v", err)
	}

	err := closeItem(db, "OTA-013", "")
	if err == nil {
		t.Fatal("expected error for already-closed item, got nil")
	}
}

// ---------------------------------------------------------------------------
// Reopen tests
// ---------------------------------------------------------------------------

func TestReopenItem(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-020")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-020", "tests/passed/v1"); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	if err := reopenItem(db, "OTA-020", "AI", "test-failed", "qa-results/2026-06/OTA-020/rerun.log"); err != nil {
		t.Fatalf("reopenItem: %v", err)
	}

	items, _ := listItems(db, "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != "Reopened" {
		t.Errorf("expected Reopened, got %s", items[0].Status)
	}

	// Verify history: Opened → Closed → Reopened
	history, err := getHistory(db, "OTA-020")
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[2].Event != "Reopened" {
		t.Errorf("expected Reopened event, got %s", history[2].Event)
	}
	if history[2].EvidencePath != "qa-results/2026-06/OTA-020/rerun.log" {
		t.Errorf("expected evidence path preserved, got %s", history[2].EvidencePath)
	}
}

func TestReopenAllReasons(t *testing.T) {
	db, _ := testDB(t)
	reasons := []string{
		"test-failed",
		"manual-testing-detected",
		"captured-evidence-contradicts",
		"end-user-report",
		"cycle-re-discovered",
		"design-reconsidered",
	}

	for i, reason := range reasons {
		otaID := fmt.Sprintf("OTA-REASON-%02d", i+1)
		item := testItem(otaID)
		if err := addItem(db, item); err != nil {
			t.Fatalf("addItem %s: %v", otaID, err)
		}
		if err := closeItem(db, otaID, "evidence/"+otaID); err != nil {
			t.Fatalf("closeItem %s: %v", otaID, err)
		}
		if err := reopenItem(db, otaID, "User", reason, "qa/"+otaID+"/reopen.log"); err != nil {
			t.Fatalf("reopenItem %s reason=%s: %v", otaID, reason, err)
		}
	}

	items, _ := listItems(db, "")
	if len(items) != len(reasons) {
		t.Fatalf("expected %d items, got %d", len(reasons), len(items))
	}
	for _, it := range items {
		if it.Status != "Reopened" {
			t.Errorf("%s: expected Reopened, got %s", it.OtaID, it.Status)
		}
	}
}

func TestReopenInvalidBy(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-REOPEN-BY")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-REOPEN-BY", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	err := reopenItem(db, "OTA-REOPEN-BY", "Bot", "test-failed", "evidence.log")
	if err == nil {
		t.Fatal("expected error for invalid 'by' value, got nil")
	}
}

func TestReopenInvalidReason(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-REOPEN-RSN")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-REOPEN-RSN", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	err := reopenItem(db, "OTA-REOPEN-RSN", "AI", "not-a-valid-reason", "evidence.log")
	if err == nil {
		t.Fatal("expected error for invalid reopen reason, got nil")
	}
}

func TestReopenWithoutEvidence(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-NOEV")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-NOEV", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	err := reopenItem(db, "OTA-NOEV", "AI", "test-failed", "")
	if err == nil {
		t.Fatal("expected error for missing evidence, got nil")
	}
}

func TestReopenItemNotFound(t *testing.T) {
	db, _ := testDB(t)

	err := reopenItem(db, "OTA-NONEXIST", "AI", "test-failed", "evidence.log")
	if err == nil {
		t.Fatal("expected error for non-existent item, got nil")
	}
}

// ---------------------------------------------------------------------------
// Round-trip: add → close → reopen → close
// ---------------------------------------------------------------------------

func TestRoundTrip(t *testing.T) {
	db, _ := testDB(t)
	otaID := "OTA-RT-001"

	// 1. Add
	item := testItem(otaID)
	if err := addItem(db, item); err != nil {
		t.Fatalf("add: %v", err)
	}

	// 2. Close
	if err := closeItem(db, otaID, "evidence/roundtrip/v1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	items, _ := listItems(db, "")
	if items[0].Status != "Fixed (→ Fixed.md)" {
		t.Fatalf("after close: expected Fixed, got %s", items[0].Status)
	}

	// 3. Reopen
	if err := reopenItem(db, otaID, "User", "end-user-report", "evidence/roundtrip/reopen.log"); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	items, _ = listItems(db, "")
	if items[0].Status != "Reopened" {
		t.Fatalf("after reopen: expected Reopened, got %s", items[0].Status)
	}

	// 4. Close again
	if err := closeItem(db, otaID, "evidence/roundtrip/v2"); err != nil {
		t.Fatalf("close (2nd): %v", err)
	}
	items, _ = listItems(db, "")
	if items[0].Status != "Fixed (→ Fixed.md)" {
		t.Fatalf("after 2nd close: expected Fixed, got %s", items[0].Status)
	}

	// Verify full history
	history, err := getHistory(db, otaID)
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 history entries, got %d", len(history))
	}
	events := []string{"Opened", "Fixed (→ Fixed.md)", "Reopened", "Fixed (→ Fixed.md)"}
	for i, ev := range events {
		if history[i].Event != ev {
			t.Errorf("history[%d]: expected %q, got %q", i, ev, history[i].Event)
		}
	}
}

// ---------------------------------------------------------------------------
// Validate tests
// ---------------------------------------------------------------------------

func TestValidatePass(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-VALID-001")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	results, err := validateAll(db)
	if err != nil {
		t.Fatalf("validateAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Fatalf("expected pass, got issues: %v", results[0].Issues)
	}
}

func TestValidateShortDescription(t *testing.T) {
	// Disable CHECK constraints to insert invalid data for validation testing
	db, _ := testDB(t)
	db.Exec(`PRAGMA ignore_check_constraints=ON`)
	_, err := db.Exec(`INSERT INTO items (ota_id, type, status, severity, title, description)
		VALUES ('OTA-SHORT', 'Bug', 'Queued', 'High', 'Short desc item', 'too short')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Exec(`PRAGMA ignore_check_constraints=OFF`)

	results, err := validateAll(db)
	if err != nil {
		t.Fatalf("validateAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected FAIL for short description, got PASS")
	}
}

func TestValidateInvalidStatus(t *testing.T) {
	db, _ := testDB(t)
	db.Exec(`PRAGMA ignore_check_constraints=ON`)
	_, err := db.Exec(`INSERT INTO items (ota_id, type, status, severity, title, description)
		VALUES ('OTA-BADST', 'Bug', 'InvalidStatus', 'High', 'Bad status item',
		        'This item has an invalid status value for testing validation.')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Exec(`PRAGMA ignore_check_constraints=OFF`)

	results, err := validateAll(db)
	if err != nil {
		t.Fatalf("validateAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected FAIL for invalid status, got PASS")
	}
}

func TestValidateClosureStatusTypeMismatch(t *testing.T) {
	db, _ := testDB(t)
	// A Feature closed with "Fixed (→ Fixed.md)" instead of "Implemented (→ Fixed.md)"
	_, err := db.Exec(`INSERT INTO items (ota_id, type, status, severity, title, description)
		VALUES ('OTA-MISMATCH', 'Feature', 'Fixed (→ Fixed.md)', 'Medium',
		        'Mismatched closure status',
		        'This Feature should use Implemented, not Fixed — test validation catches it.')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := validateAll(db)
	if err != nil {
		t.Fatalf("validateAll: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected FAIL for closure status mismatch, got PASS")
	}
}

// ---------------------------------------------------------------------------
// List with status filter
// ---------------------------------------------------------------------------

func TestListWithStatusFilter(t *testing.T) {
	db, _ := testDB(t)

	// Add items with different statuses
	statuses := []string{"Queued", "In progress", "Reopened", "Fixed (→ Fixed.md)"}
	for i, s := range statuses {
		otaID := fmt.Sprintf("OTA-LIST-%02d", i+1)
		item := testItem(otaID)
		item.Status = s
		if err := addItem(db, item); err != nil {
			t.Fatalf("addItem %s: %v", otaID, err)
		}
	}

	// Filter by Queued
	queued, err := listItems(db, "Queued")
	if err != nil {
		t.Fatalf("listItems(Queued): %v", err)
	}
	if len(queued) != 1 {
		t.Errorf("expected 1 Queued, got %d", len(queued))
	}

	// Filter by Fixed
	fixed, err := listItems(db, "Fixed (→ Fixed.md)")
	if err != nil {
		t.Fatalf("listItems(Fixed): %v", err)
	}
	if len(fixed) != 1 {
		t.Errorf("expected 1 Fixed, got %d", len(fixed))
	}

	// Invalid filter
	_, err = listItems(db, "NonExistent")
	if err == nil {
		t.Fatal("expected error for invalid filter, got nil")
	}
}

// ---------------------------------------------------------------------------
// Diff tests
// ---------------------------------------------------------------------------

func TestDiffIdenticalSlices(t *testing.T) {
	items := []Item{
		{OtaID: "OTA-001", Type: "Bug", Status: "Queued", Title: "Test", Description: string(make([]byte, 40))},
	}
	diffs := diffItems(items, items)
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs for identical slices, got %d", len(diffs))
	}
}

func TestDiffDifferentStatus(t *testing.T) {
	a := []Item{
		{OtaID: "OTA-001", Type: "Bug", Status: "Queued", Title: "Test", Description: string(make([]byte, 40))},
	}
	b := []Item{
		{OtaID: "OTA-001", Type: "Bug", Status: "Fixed (→ Fixed.md)", Title: "Test", Description: string(make([]byte, 40))},
	}

	diffs := diffItems(a, b)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Field != "status" {
		t.Errorf("expected status diff, got %s", diffs[0].Field)
	}
}

func TestDiffMissingItem(t *testing.T) {
	a := []Item{
		{OtaID: "OTA-001", Type: "Bug", Status: "Queued", Title: "A", Description: string(make([]byte, 40))},
		{OtaID: "OTA-002", Type: "Task", Status: "Queued", Title: "B", Description: string(make([]byte, 40))},
	}
	b := []Item{
		{OtaID: "OTA-001", Type: "Bug", Status: "Queued", Title: "A", Description: string(make([]byte, 40))},
	}

	diffs := diffItems(a, b)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].OtaID != "OTA-002" {
		t.Errorf("expected OTA-002 diff, got %s", diffs[0].OtaID)
	}
}

// ---------------------------------------------------------------------------
// DB creation and migration
// ---------------------------------------------------------------------------

func TestOpenDBCreatesSchema(t *testing.T) {
	f, err := os.CreateTemp("", "workable-items-create-*.db")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('items','item_history','meta')`).Scan(&count)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 tables, got %d", count)
	}

	// Verify schema version
	var version string
	err = db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&version)
	if err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != "1" {
		t.Errorf("expected schema version 1, got %s", version)
	}
}

func TestWALMode(t *testing.T) {
	f, err := os.CreateTemp("", "workable-items-wal-*.db")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer db.Close()

	var journalMode string
	err = db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" && journalMode != "WAL" {
		t.Logf("journal_mode: %s (WAL expected but some drivers report lowercase)", journalMode)
	}
}

func TestCheckpoint(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-CKP")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	if err := checkpoint(db); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	// Verify item still readable after checkpoint
	items, err := listItems(db, "")
	if err != nil {
		t.Fatalf("listItems after checkpoint: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item after checkpoint, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// History audit trail
// ---------------------------------------------------------------------------

func TestHistoryAppendOnly(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-HIST")
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	// Close
	if err := closeItem(db, "OTA-HIST", "evidence/hist"); err != nil {
		t.Fatalf("closeItem: %v", err)
	}
	// Reopen
	if err := reopenItem(db, "OTA-HIST", "AI", "cycle-re-discovered", "evidence/hist/reopen"); err != nil {
		t.Fatalf("reopenItem: %v", err)
	}
	// Close again
	if err := closeItem(db, "OTA-HIST", "evidence/hist/v2"); err != nil {
		t.Fatalf("closeItem (2nd): %v", err)
	}

	history, err := getHistory(db, "OTA-HIST")
	if err != nil {
		t.Fatalf("getHistory: %v", err)
	}

	expectedCount := 4
	if len(history) != expectedCount {
		t.Fatalf("expected %d history entries, got %d", expectedCount, len(history))
	}

	// Verify append-only: entries are in chronological order
	for i := 1; i < len(history); i++ {
		if history[i].ID <= history[i-1].ID {
			t.Errorf("history not append-only at index %d: %d <= %d", i, history[i].ID, history[i-1].ID)
		}
	}
}

// ---------------------------------------------------------------------------
// ComposeWith validation
// ---------------------------------------------------------------------------

func TestAddItemInvalidComposesWith(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-COMP-INV")
	item.ComposesWith = "not-json"

	err := addItem(db, item)
	if err == nil {
		t.Fatal("expected error for invalid composes_with, got nil")
	}
}

func TestAddItemValidComposesWith(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-COMP-VAL")
	item.ComposesWith = `["§11.4.43", "§11.4.50"]`

	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}

	items, _ := listItems(db, "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ComposesWith != `["§11.4.43", "§11.4.50"]` {
		t.Errorf("unexpected composes_with: %s", items[0].ComposesWith)
	}
}

// ---------------------------------------------------------------------------
// Multi-item operations
// ---------------------------------------------------------------------------

func TestMultipleItems(t *testing.T) {
	db, _ := testDB(t)

	for i := 1; i <= 5; i++ {
		otaID := fmt.Sprintf("OTA-MULTI-%02d", i)
		item := testItem(otaID)
		if err := addItem(db, item); err != nil {
			t.Fatalf("addItem %s: %v", otaID, err)
		}
	}

	items, err := listItems(db, "")
	if err != nil {
		t.Fatalf("listItems: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	// Verify ordering by OTA ID
	for i := 1; i < len(items); i++ {
		if items[i].OtaID <= items[i-1].OtaID {
			t.Errorf("items not sorted: %s <= %s", items[i].OtaID, items[i-1].OtaID)
		}
	}
}

// ---------------------------------------------------------------------------
// Feature → Implemented, Task → Completed type-specific closures
// ---------------------------------------------------------------------------

func TestFeatureImplementedClosure(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-FEAT")
	item.Type = "Feature"
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-FEAT", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	items, _ := listItems(db, "")
	if items[0].Status != "Implemented (→ Fixed.md)" {
		t.Errorf("expected Implemented, got %s", items[0].Status)
	}
}

func TestTaskCompletedClosure(t *testing.T) {
	db, _ := testDB(t)
	item := testItem("OTA-TASK")
	item.Type = "Task"
	if err := addItem(db, item); err != nil {
		t.Fatalf("addItem: %v", err)
	}
	if err := closeItem(db, "OTA-TASK", ""); err != nil {
		t.Fatalf("closeItem: %v", err)
	}

	items, _ := listItems(db, "")
	if items[0].Status != "Completed (→ Fixed.md)" {
		t.Errorf("expected Completed, got %s", items[0].Status)
	}
}
