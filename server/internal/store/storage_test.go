package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMemoryStoragePort_StoreAndGet(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	data := []byte("hello ota artifact")
	id, err := st.StoreArtifact(ctx, "acc-1", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("StoreArtifact: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty artifact id")
	}

	rc, err := st.GetArtifact(ctx, id)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, data) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, data)
	}

	// Get non-existent returns ErrArtifactNotFound.
	_, err = st.GetArtifact(ctx, "nonexistent")
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("expected ErrArtifactNotFound, got %v", err)
	}
}

func TestMemoryStoragePort_Delete(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	data := []byte("delete-me")
	id, err := st.StoreArtifact(ctx, "acc-1", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("StoreArtifact: %v", err)
	}

	if err := st.DeleteArtifact(ctx, id); err != nil {
		t.Fatalf("DeleteArtifact: %v", err)
	}

	_, err = st.GetArtifact(ctx, id)
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("expected ErrArtifactNotFound after delete, got %v", err)
	}

	// Delete non-existent is idempotent (no error).
	if err := st.DeleteArtifact(ctx, "nonexistent"); err != nil {
		t.Fatalf("expected no error on idempotent delete, got %v", err)
	}
}

func TestMemoryStoragePort_ListByAccount(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	for i := 0; i < 3; i++ {
		data := []byte{byte(i)}
		_, err := st.StoreArtifact(ctx, "acc-a", bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("StoreArtifact acc-a %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		data := []byte{byte(i + 100)}
		_, err := st.StoreArtifact(ctx, "acc-b", bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("StoreArtifact acc-b %d: %v", i, err)
		}
	}

	metaA, err := st.ListArtifacts(ctx, "acc-a")
	if err != nil {
		t.Fatalf("ListArtifacts acc-a: %v", err)
	}
	if len(metaA) != 3 {
		t.Fatalf("acc-a: want 3, got %d", len(metaA))
	}
	for _, m := range metaA {
		if m.AccountID != "acc-a" {
			t.Fatalf("cross-account leak: %+v", m)
		}
	}

	metaB, err := st.ListArtifacts(ctx, "acc-b")
	if err != nil {
		t.Fatalf("ListArtifacts acc-b: %v", err)
	}
	if len(metaB) != 2 {
		t.Fatalf("acc-b: want 2, got %d", len(metaB))
	}

	// Empty account returns empty slice, not nil.
	metaEmpty, err := st.ListArtifacts(ctx, "acc-empty")
	if err != nil {
		t.Fatalf("ListArtifacts acc-empty: %v", err)
	}
	if len(metaEmpty) != 0 {
		t.Fatalf("acc-empty: want 0, got %d", len(metaEmpty))
	}
}

func TestMemoryStoragePort_SizeLimit(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort().(*memoryStoragePort)
	st.maxBytes = 5

	_, err := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("123456")), 6)
	if !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("expected ErrArtifactTooLarge, got %v", err)
	}

	// At the boundary it should accept.
	_, err = st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("12345")), 5)
	if err != nil {
		t.Fatalf("expected accept at max, got %v", err)
	}
}

func TestMemoryStoragePort_EmptyAccount(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	_, err := st.StoreArtifact(ctx, "", bytes.NewReader([]byte("data")), 4)
	if !errors.Is(err, ErrStorageBackend) {
		t.Fatalf("expected ErrStorageBackend for empty account, got %v", err)
	}
}

func TestMemoryStoragePort_LargeData_ExceedsDeclaredSize(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort().(*memoryStoragePort)
	st.maxBytes = 100

	// Declare size=5 but supply 10 bytes through the reader.
	_, err := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("1234567890")), 5)
	if !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("expected ErrArtifactTooLarge when actual exceeds declared, got %v", err)
	}
}

func TestIsRejectedContentType_ELF(t *testing.T) {
	data := make([]byte, 64)
	data[0] = 0x7F
	copy(data[1:], "ELF")
	if !isRejectedContentType(data) {
		t.Fatal("ELF should be rejected")
	}
}

func TestIsRejectedContentType_PE(t *testing.T) {
	data := make([]byte, 64)
	data[0] = 'M'
	data[1] = 'Z'
	if !isRejectedContentType(data) {
		t.Fatal("PE/MZ should be rejected")
	}
}

func TestIsRejectedContentType_ShellScript(t *testing.T) {
	data := []byte("#!/bin/bash\necho hello")
	if !isRejectedContentType(data) {
		t.Fatal("shell script should be rejected")
	}
}

func TestIsRejectedContentType_MachO(t *testing.T) {
	data := make([]byte, 64)
	binary.BigEndian.PutUint32(data[0:4], 0xFEEDFACE)
	if !isRejectedContentType(data) {
		t.Fatal("Mach-O should be rejected")
	}

	binary.BigEndian.PutUint32(data[0:4], 0xCAFEBABE)
	if !isRejectedContentType(data) {
		t.Fatal("Mach-O universal/fat should be rejected")
	}
}

func TestIsRejectedContentType_PlainText(t *testing.T) {
	data := []byte("just some text with no magic bytes")
	if isRejectedContentType(data) {
		t.Fatal("plain text should not be rejected")
	}
}

func TestIsRejectedContentType_ZIP(t *testing.T) {
	// ZIP file magic: "PK\x03\x04"
	data := []byte("PK\x03\x04")
	// http.DetectContentType on PK bytes should return "application/zip",
	// which is not in the rejected set and has no executable magic prefix.
	if isRejectedContentType(data) {
		t.Fatal("ZIP should not be rejected")
	}
}

func TestIsRejectedContentType_Empty(t *testing.T) {
	if isRejectedContentType([]byte{}) {
		t.Fatal("empty data should not be rejected")
	}
}

func TestMemoryStoragePort_RejectExecutable(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	elf := make([]byte, 64)
	elf[0] = 0x7F
	copy(elf[1:], "ELF")

	_, err := st.StoreArtifact(ctx, "acc", bytes.NewReader(elf), int64(len(elf)))
	if !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("expected ErrInvalidContentType for ELF, got %v", err)
	}

	script := []byte("#!/bin/sh\ntrue")
	_, err = st.StoreArtifact(ctx, "acc", bytes.NewReader(script), int64(len(script)))
	if !errors.Is(err, ErrInvalidContentType) {
		t.Fatalf("expected ErrInvalidContentType for script, got %v", err)
	}
}

func TestMemoryStoragePort_AcceptZIP(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	// Minimal valid local file header for a stored (no compression) entry in a ZIP.
	zip := minimalZIP(t, "payload.bin", []byte("hello world"))

	id, err := st.StoreArtifact(ctx, "acc", bytes.NewReader(zip), int64(len(zip)))
	if err != nil {
		t.Fatalf("ZIP should be accepted: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestMemoryStoragePort_ArtifactMeta(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	data := []byte("artifact-content")
	id, err := st.StoreArtifact(ctx, "acc-1", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("StoreArtifact: %v", err)
	}

	meta, err := st.ListArtifacts(ctx, "acc-1")
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(meta) != 1 {
		t.Fatalf("want 1, got %d", len(meta))
	}
	m := meta[0]
	if m.ArtifactID != id {
		t.Fatalf("artifact id mismatch: %q != %q", m.ArtifactID, id)
	}
	if m.AccountID != "acc-1" {
		t.Fatalf("account id mismatch: %q", m.AccountID)
	}
	if m.Size != int64(len(data)) {
		t.Fatalf("size mismatch: %d != %d", m.Size, len(data))
	}
	if m.StoredAt.IsZero() {
		t.Fatal("stored_at is zero")
	}
}

func TestMemoryStoragePort_CrossAccountIsolation(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	dataA := []byte("acc-a-data")
	idA, err := st.StoreArtifact(ctx, "acc-a", bytes.NewReader(dataA), int64(len(dataA)))
	if err != nil {
		t.Fatalf("StoreArtifact acc-a: %v", err)
	}
	dataB := []byte("acc-b-data")
	_, err = st.StoreArtifact(ctx, "acc-b", bytes.NewReader(dataB), int64(len(dataB)))
	if err != nil {
		t.Fatalf("StoreArtifact acc-b: %v", err)
	}

	// Account A can read its own artifact.
	rc, err := st.GetArtifact(ctx, idA)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, dataA) {
		t.Fatalf("data mismatch for acc-a")
	}

	// List only returns the requesting account's artifacts.
	metaA, _ := st.ListArtifacts(ctx, "acc-a")
	if len(metaA) != 1 || metaA[0].AccountID != "acc-a" {
		t.Fatal("acc-a list leaked acc-b data")
	}

	metaB, _ := st.ListArtifacts(ctx, "acc-b")
	if len(metaB) != 1 || metaB[0].AccountID != "acc-b" {
		t.Fatal("acc-b list leaked acc-a data")
	}
}

func TestMemoryStoragePort_DeleteRemovesFromList(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	id1, _ := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("a")), 1)
	id2, _ := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("b")), 1)

	if err := st.DeleteArtifact(ctx, id1); err != nil {
		t.Fatalf("delete id1: %v", err)
	}

	meta, _ := st.ListArtifacts(ctx, "acc")
	if len(meta) != 1 {
		t.Fatalf("want 1 after delete, got %d", len(meta))
	}
	if meta[0].ArtifactID != id2 {
		t.Fatalf("expected id2, got %s", meta[0].ArtifactID)
	}
}

func TestMemoryStoragePort_Concurrent(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			data := make([]byte, 32)
			rand.Read(data)
			id, err := st.StoreArtifact(ctx, "concurrent", bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Errorf("StoreArtifact %d: %v", n, err)
				return
			}
			rc, rerr := st.GetArtifact(ctx, id)
			if rerr != nil {
				t.Errorf("GetArtifact %d: %v", n, rerr)
				return
			}
			rc.Close()
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	meta, err := st.ListArtifacts(ctx, "concurrent")
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(meta) != 10 {
		t.Fatalf("want 10 concurrent artifacts, got %d", len(meta))
	}
}

func TestMemoryStoragePort_CloseReader(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	data := []byte("data")
	id, _ := st.StoreArtifact(ctx, "acc", bytes.NewReader(data), int64(len(data)))

	rc, err := st.GetArtifact(ctx, id)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// NopCloser returns nil on close, verified above.
}

func TestMemoryStoragePort_ArtifactIDUniqueness(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte{byte(i)}), 1)
		if err != nil {
			t.Fatalf("StoreArtifact %d: %v", i, err)
		}
		if ids[id] {
			t.Fatalf("duplicate id generated: %s", id)
		}
		ids[id] = true
	}
}

func TestMemoryStoragePort_ReadCloserReadFull(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	data := []byte("full content read")
	id, _ := st.StoreArtifact(ctx, "acc", bytes.NewReader(data), int64(len(data)))

	rc, err := st.GetArtifact(ctx, id)
	if err != nil {
		t.Fatalf("GetArtifact: %v", err)
	}

	buf := make([]byte, len(data))
	n, err := io.ReadFull(rc, buf)
	if err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if n != len(data) || !bytes.Equal(buf, data) {
		t.Fatalf("ReadFull mismatch: got %d bytes %q", n, buf)
	}
	rc.Close()
}

func TestMemoryStoragePort_ManyArtifactsList(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	const n = 50
	for i := 0; i < n; i++ {
		_, err := st.StoreArtifact(ctx, "acc", strings.NewReader("x"), 1)
		if err != nil {
			t.Fatalf("StoreArtifact %d: %v", i, err)
		}
	}

	meta, err := st.ListArtifacts(ctx, "acc")
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(meta) != n {
		t.Fatalf("want %d, got %d", n, len(meta))
	}
}

func TestStoragePort_StoreTimeResolution(t *testing.T) {
	ctx := context.Background()
	st := NewMemoryStoragePort()

	before := time.Now().UTC()
	id, _ := st.StoreArtifact(ctx, "acc", bytes.NewReader([]byte("t")), 1)
	after := time.Now().UTC()

	meta, _ := st.ListArtifacts(ctx, "acc")
	if len(meta) != 1 {
		t.Fatal("expected 1 artifact")
	}
	stored := meta[0].StoredAt
	if stored.Before(before) || stored.After(after) {
		t.Fatalf("stored_at %v not between %v and %v", stored, before, after)
	}
	_ = id
}

// minimalZIP returns a valid bare-minimum ZIP archive containing one stored entry.
func minimalZIP(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer

	// Local file header: signature + version + flags + method + timestamps +
	// crc32 + compressed/uncompressed size + filename len + extra len + filename.
	buf.Write([]byte("PK\x03\x04"))                     // signature
	buf.Write([]byte{0x14, 0x00})                      // version needed
	buf.Write([]byte{0x00, 0x00})                      // flags
	buf.Write([]byte{0x00, 0x00})                      // method (stored)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})          // mod time/date
	crc := uint32(0)
	for _, b := range payload {
		crc = crc32Update(crc, b)
	}
	binary.LittleEndian.PutUint32(buf.Bytes()[14:18], crc) // crc32 — we write into the buffer after extending
	// Actually we already wrote 14 bytes. Let me use a different approach.
	_ = crc // crc is not critical for structure detection
	buf.Reset()

	// Simpler approach: build the header with correct fields.
	var w bytes.Buffer
	w.Write([]byte("PK\x03\x04"))             // signature
	w.Write([]byte{0x14, 0x00})               // version needed (2.0)
	w.Write([]byte{0x00, 0x00})               // flags
	w.Write([]byte{0x00, 0x00})               // method (stored)
	w.Write([]byte{0x00, 0x00, 0x00, 0x00})   // mod time/date
	crc32B := make([]byte, 4)                  // placeholder crc32
	binary.LittleEndian.PutUint32(crc32B, 0)
	w.Write(crc32B)
	compSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(compSize, uint32(len(payload)))
	w.Write(compSize)                         // compressed size
	w.Write(compSize)                         // uncompressed size
	fnLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(fnLen, uint16(len(name)))
	w.Write(fnLen)                            // filename length
	w.Write([]byte{0x00, 0x00})               // extra field length
	w.Write([]byte(name))                     // filename
	w.Write(payload)                          // file data

	return w.Bytes()
}

func crc32Update(crc uint32, b byte) uint32 {
	crc ^= uint32(b)
	for i := 0; i < 8; i++ {
		if crc&1 != 0 {
			crc = (crc >> 1) ^ 0xEDB88320
		} else {
			crc >>= 1
		}
	}
	return crc
}
