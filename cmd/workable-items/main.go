// Workable-items CLI — single-source-of-truth DB for §11.4.93/§11.4.95
//
// Maintains a SQLite-backed workable-items database at docs/workable_items.db
// (TRACKED in git per §11.4.95, NEVER gitignored).
//
// Commands:
//
//	add      Insert a new workable item
//	close    Close an item with type-appropriate closure vocabulary (§11.4.33)
//	reopen   Reopen an item with reason + evidence (§11.4.34)
//	list     List items, optionally filtered by status
//	sync     Bidirectional markdown <-> DB sync
//	validate Check schema integrity constraints
//	diff     Show differences between MD and DB state
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	defaultDBPath = "docs/workable_items.db"
	schemaVersion = 1
)

// Status values (closed vocabulary per §11.4.15, extended per §11.4.33, §11.4.90)
var validStatuses = map[string]bool{
	"Queued":                   true,
	"In progress":              true,
	"Ready for testing":        true,
	"In testing":               true,
	"Reopened":                 true,
	"Operator-blocked":         true,
	"Fixed (→ Fixed.md)":       true,
	"Implemented (→ Fixed.md)": true,
	"Completed (→ Fixed.md)":   true,
	"Obsolete (→ Fixed.md)":    true,
}

// Valid closure statuses per Type (§11.4.33)
var closureStatusByType = map[string]string{
	"Bug":     "Fixed (→ Fixed.md)",
	"Feature": "Implemented (→ Fixed.md)",
	"Task":    "Completed (→ Fixed.md)",
}

// Valid statuses for terminal (closed) items
var terminalStatuses = map[string]bool{
	"Fixed (→ Fixed.md)":       true,
	"Implemented (→ Fixed.md)": true,
	"Completed (→ Fixed.md)":   true,
	"Obsolete (→ Fixed.md)":    true,
}

// Valid types (§11.4.16)
var validTypes = map[string]bool{
	"Bug":     true,
	"Feature": true,
	"Task":    true,
}

// Valid severities
var validSeverities = map[string]bool{
	"Critical": true,
	"High":     true,
	"Medium":   true,
	"Low":      true,
}

// Close-set reopen reasons (§11.4.34)
var validReopenReasons = map[string]bool{
	"test-failed":                   true,
	"manual-testing-detected":       true,
	"captured-evidence-contradicts": true,
	"end-user-report":               true,
	"cycle-re-discovered":           true,
	"design-reconsidered":           true,
}

// Reopen closed-vocabulary "By" values (§11.4.34)
func isValidReopenBy(v string) bool {
	return v == "AI" || v == "User"
}

// Event types for item_history
var validHistoryEvents = map[string]bool{
	"Opened":                   true,
	"Reopened":                 true,
	"Fixed (→ Fixed.md)":       true,
	"Implemented (→ Fixed.md)": true,
	"Completed (→ Fixed.md)":   true,
	"Obsolete (→ Fixed.md)":    true,
	"Operator-blocked":         true,
	"Status Update":            true,
}

// ---------------------------------------------------------------------------
// Item struct
// ---------------------------------------------------------------------------

// Item represents a row in the items table.
type Item struct {
	OtaID           string `json:"ota_id"`
	Type            string `json:"type"`
	Status          string `json:"status"`
	Severity        string `json:"severity"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	ComposesWith    string `json:"composes_with"` // JSON array
	CurrentLocation string `json:"current_location"`
	CreatedAt       string `json:"created_at"`
	ModifiedAt      string `json:"modified_at"`
}

// HistoryEntry represents a row in item_history.
type HistoryEntry struct {
	ID           int    `json:"id"`
	OtaID        string `json:"ota_id"`
	By           string `json:"by"`
	Event        string `json:"event"`
	OnDate       string `json:"on_date"`
	Reason       string `json:"reason"`
	EvidencePath string `json:"evidence_path"`
	Outcome      string `json:"outcome"`
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

// openDB opens (or creates) the SQLite database at path and runs migrations.
func openDB(dbPath string) (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}

	// WAL mode (§11.4.95: checkpoint before commit)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Schema migration
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// migrate ensures the schema exists and is at the expected version.
func migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_version', '1')`,

		`CREATE TABLE IF NOT EXISTS items (
			ota_id TEXT PRIMARY KEY,
			type TEXT NOT NULL CHECK(type IN ('Bug','Feature','Task')),
			status TEXT NOT NULL DEFAULT 'Queued'
				CHECK(status IN ('Queued','In progress','Ready for testing','In testing',
				'Reopened','Operator-blocked',
				'Fixed (→ Fixed.md)','Implemented (→ Fixed.md)',
				'Completed (→ Fixed.md)','Obsolete (→ Fixed.md)')),
			severity TEXT NOT NULL DEFAULT 'Medium'
				CHECK(severity IN ('Critical','High','Medium','Low')),
			title TEXT NOT NULL,
			description TEXT NOT NULL CHECK(length(description) >= 40),
			composes_with TEXT NOT NULL DEFAULT '[]',
			current_location TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			modified_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS item_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ota_id TEXT NOT NULL REFERENCES items(ota_id),
			by TEXT NOT NULL CHECK(by IN ('AI','User')),
			event TEXT NOT NULL
				CHECK(event IN ('Opened','Reopened',
				'Fixed (→ Fixed.md)','Implemented (→ Fixed.md)',
				'Completed (→ Fixed.md)','Obsolete (→ Fixed.md)',
				'Operator-blocked','Status Update')),
			on_date TEXT NOT NULL DEFAULT (datetime('now')),
			reason TEXT NOT NULL DEFAULT '',
			evidence_path TEXT NOT NULL DEFAULT '',
			outcome TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE INDEX IF NOT EXISTS idx_history_ota_id ON item_history(ota_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_status ON items(status)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration: %s: %w", m[:50], err)
		}
	}
	return nil
}

// checkpoint runs WAL checkpoint to ensure clean state before commit (§11.4.95).
func checkpoint(db *sql.DB) error {
	_, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// ---------------------------------------------------------------------------
// CRUD operations
// ---------------------------------------------------------------------------

// addItem inserts a new workable item.
func addItem(db *sql.DB, item *Item) error {
	if !validTypes[item.Type] {
		return fmt.Errorf("invalid type %q; must be Bug, Feature, or Task", item.Type)
	}
	if !validStatuses[item.Status] {
		return fmt.Errorf("invalid status %q", item.Status)
	}
	if !validSeverities[item.Severity] {
		return fmt.Errorf("invalid severity %q", item.Severity)
	}
	if item.Title == "" {
		return errors.New("title is required")
	}
	if len(item.Description) < 40 {
		return fmt.Errorf("description must be >= 40 characters (got %d)", len(item.Description))
	}
	if item.OtaID == "" {
		return errors.New("ota_id is required (e.g. OTA-NNN)")
	}

	// Validate composes_with is valid JSON
	var dummy []interface{}
	if err := json.Unmarshal([]byte(item.ComposesWith), &dummy); err != nil {
		return fmt.Errorf("composes_with must be a JSON array: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	item.CreatedAt = now
	item.ModifiedAt = now

	_, err := db.Exec(`INSERT INTO items
		(ota_id, type, status, severity, title, description, composes_with, current_location, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.OtaID, item.Type, item.Status, item.Severity, item.Title,
		item.Description, item.ComposesWith, item.CurrentLocation, item.CreatedAt, item.ModifiedAt)
	if err != nil {
		return fmt.Errorf("insert item %s: %w", item.OtaID, err)
	}

	// History entry: Opened
	return addHistory(db, item.OtaID, "AI", "Opened", "Item created", "", "")
}

// closeItem closes an item with type-appropriate closure vocabulary (§11.4.33).
func closeItem(db *sql.DB, otaID string, evidence string) error {
	var typ, currentStatus string
	err := db.QueryRow(`SELECT type, status FROM items WHERE ota_id = ?`, otaID).Scan(&typ, &currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("item %s not found", otaID)
		}
		return fmt.Errorf("query item %s: %w", otaID, err)
	}

	if terminalStatuses[currentStatus] {
		return fmt.Errorf("item %s is already closed (status: %s)", otaID, currentStatus)
	}

	closureStatus, ok := closureStatusByType[typ]
	if !ok {
		return fmt.Errorf("unknown type %q for closure vocabulary", typ)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err = db.Exec(`UPDATE items SET status = ?, modified_at = ? WHERE ota_id = ?`,
		closureStatus, now, otaID)
	if err != nil {
		return fmt.Errorf("close item %s: %w", otaID, err)
	}

	return addHistory(db, otaID, "AI", closureStatus, "Closed per §11.4.33", evidence, "")
}

// reopenItem reopens an item with §11.4.34 details.
func reopenItem(db *sql.DB, otaID, by, reason, evidence string) error {
	if !isValidReopenBy(by) {
		return fmt.Errorf("invalid 'by' value %q; must be AI or User", by)
	}
	if !validReopenReasons[reason] {
		reasons := make([]string, 0, len(validReopenReasons))
		for r := range validReopenReasons {
			reasons = append(reasons, r)
		}
		sort.Strings(reasons)
		return fmt.Errorf("invalid reopen reason %q; valid: %s", reason, strings.Join(reasons, ", "))
	}
	if evidence == "" {
		return errors.New("evidence path is required for reopen (§11.4.34)")
	}

	var currentStatus string
	err := db.QueryRow(`SELECT status FROM items WHERE ota_id = ?`, otaID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("item %s not found", otaID)
		}
		return fmt.Errorf("query item %s: %w", otaID, err)
	}

	if currentStatus != "Reopened" && !terminalStatuses[currentStatus] {
		return fmt.Errorf("item %s is not closed or reopened (current: %s)", otaID, currentStatus)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err = db.Exec(`UPDATE items SET status = 'Reopened', modified_at = ? WHERE ota_id = ?`,
		now, otaID)
	if err != nil {
		return fmt.Errorf("reopen item %s: %w", otaID, err)
	}

	reasonDetail := fmt.Sprintf("%s: %s", by, reason)
	return addHistory(db, otaID, by, "Reopened", reasonDetail, evidence, "")
}

// addHistory appends an entry to item_history.
func addHistory(db *sql.DB, otaID, by, event, reason, evidencePath, outcome string) error {
	if !validHistoryEvents[event] {
		return fmt.Errorf("invalid history event %q", event)
	}
	_, err := db.Exec(`INSERT INTO item_history
		(ota_id, by, event, on_date, reason, evidence_path, outcome)
		VALUES (?, ?, ?, datetime('now'), ?, ?, ?)`,
		otaID, by, event, reason, evidencePath, outcome)
	return err
}

// listItems queries items with optional status filter.
func listItems(db *sql.DB, statusFilter string) ([]Item, error) {
	query := `SELECT ota_id, type, status, severity, title, description,
		composes_with, current_location, created_at, modified_at FROM items`
	var args []interface{}
	if statusFilter != "" {
		if !validStatuses[statusFilter] {
			return nil, fmt.Errorf("invalid status filter %q", statusFilter)
		}
		query += " WHERE status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY ota_id"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.OtaID, &it.Type, &it.Status, &it.Severity,
			&it.Title, &it.Description, &it.ComposesWith,
			&it.CurrentLocation, &it.CreatedAt, &it.ModifiedAt); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// getHistory returns the history for a given OTA ID.
func getHistory(db *sql.DB, otaID string) ([]HistoryEntry, error) {
	rows, err := db.Query(`SELECT id, ota_id, by, event, on_date, reason, evidence_path, outcome
		FROM item_history WHERE ota_id = ? ORDER BY id`, otaID)
	if err != nil {
		return nil, fmt.Errorf("history query: %w", err)
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var h HistoryEntry
		if err := rows.Scan(&h.ID, &h.OtaID, &h.By, &h.Event, &h.OnDate,
			&h.Reason, &h.EvidencePath, &h.Outcome); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		entries = append(entries, h)
	}
	return entries, rows.Err()
}

// ---------------------------------------------------------------------------
// Validation (§11.4.93: validate command)
// ---------------------------------------------------------------------------

// ValidationResult holds per-item validation issues.
type ValidationResult struct {
	OtaID  string
	Issues []string
	Pass   bool
}

// validateAll checks every item against schema integrity constraints.
func validateAll(db *sql.DB) ([]ValidationResult, error) {
	items, err := listItems(db, "")
	if err != nil {
		return nil, err
	}

	var results []ValidationResult
	for _, it := range items {
		var issues []string

		if !validTypes[it.Type] {
			issues = append(issues, fmt.Sprintf("invalid type: %q", it.Type))
		}
		if !validStatuses[it.Status] {
			issues = append(issues, fmt.Sprintf("invalid status: %q", it.Status))
		}
		if len(it.Title) == 0 {
			issues = append(issues, "title is empty")
		}
		if len(it.Description) < 40 {
			issues = append(issues, fmt.Sprintf("description too short (%d chars, need >=40)", len(it.Description)))
		}
		if !validSeverities[it.Severity] {
			issues = append(issues, fmt.Sprintf("invalid severity: %q", it.Severity))
		}

		// Verify composes_with is valid JSON
		var dummy []interface{}
		if err := json.Unmarshal([]byte(it.ComposesWith), &dummy); err != nil {
			issues = append(issues, fmt.Sprintf("composes_with is not valid JSON: %s", err))
		}

		// Verify closure status matches type for closed items (§11.4.33)
		if terminalStatuses[it.Status] {
			expected, ok := closureStatusByType[it.Type]
			if ok && it.Status != expected && it.Status != "Obsolete (→ Fixed.md)" {
				issues = append(issues, fmt.Sprintf("closure status %q does not match type %q (expected %q or Obsolete)",
					it.Status, it.Type, expected))
			}
		}

		results = append(results, ValidationResult{
			OtaID:  it.OtaID,
			Issues: issues,
			Pass:   len(issues) == 0,
		})
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Diff: compares two item slices structurally
// ---------------------------------------------------------------------------

// DiffResult captures a single difference.
type DiffResult struct {
	OtaID  string
	Field  string
	DBA    string
	DBNotB string
}

// diffItems compares a and b item slices.
func diffItems(a, b []Item) []DiffResult {
	indexA := make(map[string]Item)
	indexB := make(map[string]Item)
	for _, it := range a {
		indexA[it.OtaID] = it
	}
	for _, it := range b {
		indexB[it.OtaID] = it
	}

	var diffs []DiffResult

	// Items in A but not B (or different)
	for id, itemA := range indexA {
		itemB, ok := indexB[id]
		if !ok {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "EXISTS", DBA: "yes", DBNotB: "no"})
			continue
		}
		if itemA.Type != itemB.Type {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "type", DBA: itemA.Type, DBNotB: itemB.Type})
		}
		if itemA.Status != itemB.Status {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "status", DBA: itemA.Status, DBNotB: itemB.Status})
		}
		if itemA.Title != itemB.Title {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "title", DBA: itemA.Title, DBNotB: itemB.Title})
		}
		if itemA.Description != itemB.Description {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "description", DBA: itemA.Description, DBNotB: itemB.Description})
		}
	}

	// Items in B but not A
	for id := range indexB {
		if _, ok := indexA[id]; !ok {
			diffs = append(diffs, DiffResult{OtaID: id, Field: "EXISTS", DBA: "no", DBNotB: "yes"})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].OtaID != diffs[j].OtaID {
			return diffs[i].OtaID < diffs[j].OtaID
		}
		return diffs[i].Field < diffs[j].Field
	})

	return diffs
}

// ---------------------------------------------------------------------------
// Sync: MD <-> DB (bidirectional stub / framework)
// ---------------------------------------------------------------------------

// mdParseItem extracts a minimal Item from a markdown heading.
// This is a framework for the md-to-db direction — real parsing
// depends on the actual Issues.md format.
func mdParseItem(heading string) *Item {
	// Format: ## §X.Y. [OTA-NNN] Title
	// Extract OTA ID and title from heading
	heading = strings.TrimSpace(heading)

	if !strings.HasPrefix(heading, "## ") {
		return nil
	}
	body := strings.TrimPrefix(heading, "## ")

	// Extract OTA-NNN
	start := strings.Index(body, "[OTA-")
	if start < 0 {
		return nil
	}
	end := strings.Index(body[start:], "]")
	if end < 0 {
		return nil
	}
	otaID := body[start+1 : start+end]

	// Extract title (text after the ATM bracket)
	title := strings.TrimSpace(body[start+end+1:])

	if otaID == "" || title == "" {
		return nil
	}

	return &Item{
		OtaID:  otaID,
		Title:  title,
		Status: "Queued",
		Type:   "Task",
	}
}

// ---------------------------------------------------------------------------
// CLI entry point
// ---------------------------------------------------------------------------

func usage() {
	fmt.Fprintf(os.Stderr, `Workable-items CLI — single-source-of-truth DB (§11.4.93, §11.4.95)

Usage:
  workable-items <command> [flags]

Commands:
  add      Insert a new workable item
  close    Close an item with type-appropriate vocabulary (§11.4.33)
  reopen   Reopen an item (§11.4.34)
  list     List items (optional status filter)
  sync     Bidirectional MD <-> DB sync
  validate Check schema integrity for all items
  diff     Show differences between two states

Flags:
  --db path   Path to SQLite DB (default: docs/workable_items.db)

Run "workable-items <command> --help" for command-specific flags.
`)
}

func main() {
	// Skip global flags (--db) to find the subcommand
	cmdIndex := 1
	for cmdIndex < len(os.Args) && os.Args[cmdIndex] == "--db" {
		cmdIndex += 2
	}
	if cmdIndex >= len(os.Args) {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[cmdIndex]
	args := os.Args[cmdIndex+1:]

	switch cmd {
	case "add":
		runAdd(args)
	case "close":
		runClose(args)
	case "reopen":
		runReopen(args)
	case "list":
		runList(args)
	case "sync":
		runSync(args)
	case "validate":
		runValidate(args)
	case "diff":
		runDiff(args)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

// parseCommonFlags extracts --db from args, returning remaining args.
func parseCommonFlags(args []string) (dbPath string, remaining []string) {
	dbPath = defaultDBPath
	for i := 0; i < len(args); i++ {
		if args[i] == "--db" && i+1 < len(args) {
			dbPath = args[i+1]
			i++
			continue
		}
		remaining = append(remaining, args[i])
	}
	return
}

// ---------------------------------------------------------------------------
// Command: add
// ---------------------------------------------------------------------------

func runAdd(args []string) {
	var item Item
	item.Status = "Queued"
	item.Severity = "Medium"
	item.ComposesWith = "[]"

	parseFlags := func() {
		for i := 0; i < len(args); i++ {
			switch {
			case args[i] == "--db" && i+1 < len(args):
				// handled by parseCommonFlags
				i++
			case args[i] == "--id" && i+1 < len(args):
				item.OtaID = args[i+1]
				i++
			case args[i] == "--type" && i+1 < len(args):
				item.Type = args[i+1]
				i++
			case args[i] == "--status" && i+1 < len(args):
				item.Status = args[i+1]
				i++
			case args[i] == "--severity" && i+1 < len(args):
				item.Severity = args[i+1]
				i++
			case args[i] == "--title" && i+1 < len(args):
				item.Title = args[i+1]
				i++
			case args[i] == "--desc" && i+1 < len(args):
				item.Description = args[i+1]
				i++
			case args[i] == "--location" && i+1 < len(args):
				item.CurrentLocation = args[i+1]
				i++
			case args[i] == "--composes" && i+1 < len(args):
				item.ComposesWith = args[i+1]
				i++
			}
		}
	}
	parseFlags()

	dbPath, _ := parseCommonFlags(args)
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := addItem(db, &item); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if err := checkpoint(db); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: checkpoint: %v\n", err)
	}

	fmt.Printf("OK: added %s (%s) at %s\n", item.OtaID, item.Type, item.CreatedAt)
}

// ---------------------------------------------------------------------------
// Command: close
// ---------------------------------------------------------------------------

func runClose(args []string) {
	var otaID, evidence string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			i++
		case args[i] == "--id" && i+1 < len(args):
			otaID = args[i+1]
			i++
		case args[i] == "--evidence" && i+1 < len(args):
			evidence = args[i+1]
			i++
		}
	}

	if otaID == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --id is required")
		os.Exit(1)
	}

	dbPath, _ := parseCommonFlags(args)
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := closeItem(db, otaID, evidence); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if err := checkpoint(db); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: checkpoint: %v\n", err)
	}

	fmt.Printf("OK: closed %s\n", otaID)
}

// ---------------------------------------------------------------------------
// Command: reopen
// ---------------------------------------------------------------------------

func runReopen(args []string) {
	var otaID, by, reason, evidence string
	by = "AI" // default

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			i++
		case args[i] == "--id" && i+1 < len(args):
			otaID = args[i+1]
			i++
		case args[i] == "--by" && i+1 < len(args):
			by = args[i+1]
			i++
		case args[i] == "--reason" && i+1 < len(args):
			reason = args[i+1]
			i++
		case args[i] == "--evidence" && i+1 < len(args):
			evidence = args[i+1]
			i++
		}
	}

	if otaID == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --id is required")
		os.Exit(1)
	}
	if reason == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --reason is required")
		os.Exit(1)
	}
	if evidence == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --evidence is required (§11.4.34)")
		os.Exit(1)
	}

	dbPath, _ := parseCommonFlags(args)
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := reopenItem(db, otaID, by, reason, evidence); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if err := checkpoint(db); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: checkpoint: %v\n", err)
	}

	fmt.Printf("OK: reopened %s (%s: %s)\n", otaID, by, reason)
}

// ---------------------------------------------------------------------------
// Command: list
// ---------------------------------------------------------------------------

func runList(args []string) {
	var statusFilter string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--db" && i+1 < len(args):
			i++
		case (args[i] == "--status" || args[i] == "-s") && i+1 < len(args):
			statusFilter = args[i+1]
			i++
		}
	}

	dbPath, _ := parseCommonFlags(args)
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	items, err := listItems(db, statusFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "OTA ID\tType\tStatus\tSeverity\tTitle")
	fmt.Fprintln(w, "------\t----\t------\t--------\t-----")
	for _, it := range items {
		shortTitle := it.Title
		if len(shortTitle) > 60 {
			shortTitle = shortTitle[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", it.OtaID, it.Type, it.Status, it.Severity, shortTitle)
	}
	w.Flush()
	fmt.Printf("\n%d item(s)\n", len(items))
}

// ---------------------------------------------------------------------------
// Command: sync
// ---------------------------------------------------------------------------

func runSync(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "ERROR: usage: workable-items sync <md-to-db|db-to-md>")
		os.Exit(1)
	}

	subCmd := args[0]
	dbPath, _ := parseCommonFlags(args)

	switch subCmd {
	case "md-to-db":
		// Stub: reads Issues.md headings and upserts into DB.
		// In a full implementation this parses the full Issues.md/Fixed.md structure.
		issuesPath := "docs/Issues.md"
		data, err := os.ReadFile(issuesPath)
		if err != nil {
			// Issues.md may not exist yet in every repo
			fmt.Fprintf(os.Stderr, "WARN: cannot read %s: %v\n", issuesPath, err)
			os.Exit(0)
		}

		db, err := openDB(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		lines := strings.Split(string(data), "\n")
		count := 0
		for _, line := range lines {
			item := mdParseItem(line)
			if item == nil {
				continue
			}
			// Upsert: insert or update title
			_, err := db.Exec(`INSERT INTO items (ota_id, type, status, severity, title, description, composes_with, created_at)
				VALUES (?, 'Task', 'Queued', 'Medium', ?, '(synced from Issues.md)', '[]', datetime('now'))
				ON CONFLICT(ota_id) DO UPDATE SET title = excluded.title`,
				item.OtaID, item.Title)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: upsert %s: %v\n", item.OtaID, err)
				continue
			}
			count++
		}

		if err := checkpoint(db); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: checkpoint: %v\n", err)
		}
		fmt.Printf("OK: synced %d items from %s\n", count, issuesPath)

	case "db-to-md":
		fmt.Println("WARN: db-to-md sync is a stub (real implementation generates Issues.md from DB)")
		fmt.Println("Use the generate_issues_summary.sh generator with DB as source.")

	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown sync direction: %s (use md-to-db or db-to-md)\n", subCmd)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Command: validate
// ---------------------------------------------------------------------------

func runValidate(args []string) {
	dbPath, _ := parseCommonFlags(args)
	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	results, err := validateAll(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		} else {
			failed++
			fmt.Fprintf(os.Stderr, "FAIL %s:\n", r.OtaID)
			for _, issue := range r.Issues {
				fmt.Fprintf(os.Stderr, "  - %s\n", issue)
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed out of %d items\n", passed, failed, len(results))
	if failed > 0 {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Command: diff
// ---------------------------------------------------------------------------

func runDiff(args []string) {
	dbPath, _ := parseCommonFlags(args)

	// Diff mode: compare two DB snapshots via --with flag, or compare
	// DB state against Issues.md headings.
	var withPath string
	for i := 0; i < len(args); i++ {
		if (args[i] == "--with" || args[i] == "-w") && i+1 < len(args) {
			withPath = args[i+1]
			break
		}
	}

	if withPath != "" {
		// Compare two DB files
		dbA, err := openDB(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		defer dbA.Close()

		dbB, err := openDB(withPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		defer dbB.Close()

		itemsA, _ := listItems(dbA, "")
		itemsB, _ := listItems(dbB, "")

		diffs := diffItems(itemsA, itemsB)
		if len(diffs) == 0 {
			fmt.Println("OK: databases are identical")
			return
		}
		fmt.Printf("%d difference(s) found:\n", len(diffs))
		for _, d := range diffs {
			fmt.Printf("  %s %s: %q vs %q\n", d.OtaID, d.Field, d.DBA, d.DBNotB)
		}
		return
	}

	// Compare DB against Issues.md headings
	issuesPath := "docs/Issues.md"
	data, err := os.ReadFile(issuesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: cannot read %s: %v\n", issuesPath, err)
		os.Exit(0)
	}

	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	items, err := listItems(db, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	mdItems := make([]Item, 0)
	seen := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parsed := mdParseItem(line)
		if parsed == nil || seen[parsed.OtaID] {
			continue
		}
		seen[parsed.OtaID] = true
		mdItems = append(mdItems, *parsed)
	}

	diffs := diffItems(items, mdItems)
	if len(diffs) == 0 {
		fmt.Println("OK: DB and Issues.md are in sync")
		return
	}
	fmt.Printf("%d difference(s) between DB and Issues.md:\n", len(diffs))
	for _, d := range diffs {
		fmt.Printf("  %s %s: DB=%q MD=%q\n", d.OtaID, d.Field, d.DBA, d.DBNotB)
	}
}
