package store

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"time"
)

var (
	// ErrArtifactTooLarge is returned by StoragePort.StoreArtifact when the
	// supplied data exceeds the configured maximum artifact size.
	ErrArtifactTooLarge = errors.New("store: artifact exceeds maximum allowed size")
	// ErrInvalidContentType is returned by StoragePort.StoreArtifact when the
	// detected content type is an executable or otherwise disallowed format.
	ErrInvalidContentType = errors.New("store: artifact content type is not allowed")
	// ErrArtifactNotFound is returned by StoragePort.GetArtifact when the
	// requested artifact does not exist.
	ErrArtifactNotFound = errors.New("store: artifact not found in object storage")
	// ErrStorageBackend is returned when the storage backend encounters an
	// unrecoverable internal error.
	ErrStorageBackend = errors.New("store: storage backend error")
)

// DefaultMaxArtifactBytes is the default per-artifact size cap enforced at the
// storage seam (defense-in-depth; the HTTP layer also enforces Helix_MAX_UPLOAD_BYTES).
const DefaultMaxArtifactBytes int64 = 2 << 30 // 2 GiB

// ArtifactMeta describes one stored artifact blob in the object storage seam.
// AccountID scopes the artifact to the tenant that uploaded it.
type ArtifactMeta struct {
	ArtifactID  string
	AccountID   string
	Size        int64
	ContentType string
	StoredAt    time.Time
}

// StoragePort is the object-storage seam for OTA artifact blobs.
// One interface, multiple backends: in-memory (dev/testing/graceful degradation),
// MinIO/S3 (production). Every artifact is scoped to the account that uploaded it.
type StoragePort interface {
	// StoreArtifact persists data as a new artifact blob owned by accountID.
	// Returns the assigned artifact ID. Rejects oversized payloads
	// (ErrArtifactTooLarge) and executables/disallowed content types
	// (ErrInvalidContentType).
	StoreArtifact(ctx context.Context, accountID string, data io.Reader, size int64) (artifactID string, err error)

	// GetArtifact returns a reader for the artifact blob. The caller must close
	// the returned ReadCloser. Returns ErrArtifactNotFound when the artifact
	// does not exist.
	GetArtifact(ctx context.Context, artifactID string) (io.ReadCloser, error)

	// DeleteArtifact removes an artifact blob. Idempotent — deleting a
	// non-existent artifact is a no-op, not an error.
	DeleteArtifact(ctx context.Context, artifactID string) error

	// ListArtifacts returns metadata for every artifact owned by accountID.
	ListArtifacts(ctx context.Context, accountID string) ([]ArtifactMeta, error)
}

// --- content-type validation (security hardening) ---

// detectContentType returns the best-guess MIME type for the first 512 bytes.
func detectContentType(data []byte) string {
	if len(data) > 512 {
		return http.DetectContentType(data[:512])
	}
	return http.DetectContentType(data)
}

// isRejectedContentType reports whether data is an executable or otherwise
// disallowed format. It checks both the MIME type (from http.DetectContentType)
// and magic-byte signatures for formats the stdlib detector may miss
// (ELF, PE, Mach-O, shell scripts).
func isRejectedContentType(data []byte) bool {
	ct := detectContentType(data)

	// Reject MIME types that indicate executable code.
	switch ct {
	case "application/x-executable",
		"application/x-msdownload",
		"application/x-dosexec",
		"application/x-elf",
		"application/x-sharedlib",
		"application/x-mach-binary":
		return true
	}

	if len(data) < 2 {
		return false
	}

	// Magic-byte checks for executables the stdlib detector often returns as
	// "application/octet-stream" or "text/plain".
	if data[0] == 0x7F && data[1] == 'E' && data[2] == 'L' && data[3] == 'F' {
		return true // ELF
	}
	if data[0] == 'M' && data[1] == 'Z' {
		return true // PE / DOS executable
	}

	// Shell scripts start with #! shebang.
	if data[0] == '#' && data[1] == '!' {
		return true
	}

	// Mach-O fat/universal binary magic (big-endian 32-bit).
	if len(data) >= 4 {
		magic := binary.BigEndian.Uint32(data[0:4])
		switch magic {
		case 0xFEEDFACE, // Mach-O 32-bit
			0xFEEDFACF, // Mach-O 64-bit
			0xCAFEBABE, // Universal / fat binary
			0xBEBAFECA: // Universal / fat binary reverse
			return true
		}
	}

	return false
}
