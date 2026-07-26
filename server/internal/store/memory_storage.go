package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

// memoryStoragePort is an in-memory, concurrency-safe StoragePort
// implementation. It backs dev, testing, and graceful-degradation scenarios.
type memoryStoragePort struct {
	mu        sync.RWMutex
	artifacts map[string]storedBlob
	byAccount map[string][]string // accountID -> ordered artifactIDs
	maxBytes  int64
}

type storedBlob struct {
	meta ArtifactMeta
	data []byte
}

// NewMemoryStoragePort constructs an empty in-memory storage backend.
func NewMemoryStoragePort() StoragePort {
	return &memoryStoragePort{
		artifacts: make(map[string]storedBlob),
		byAccount: make(map[string][]string),
		maxBytes:  DefaultMaxArtifactBytes,
	}
}

func genStorageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *memoryStoragePort) StoreArtifact(_ context.Context, accountID string, data io.Reader, size int64) (string, error) {
	if accountID == "" {
		return "", ErrStorageBackend
	}
	if size > m.maxBytes {
		return "", ErrArtifactTooLarge
	}

	buf, err := io.ReadAll(io.LimitReader(data, size+1))
	if err != nil {
		return "", ErrStorageBackend
	}
	if int64(len(buf)) > size {
		return "", ErrArtifactTooLarge
	}
	if int64(len(buf)) > m.maxBytes {
		return "", ErrArtifactTooLarge
	}

	if isRejectedContentType(buf) {
		return "", ErrInvalidContentType
	}

	id := genStorageID()
	ct := detectContentType(buf)
	now := time.Now().UTC()

	m.mu.Lock()
	m.artifacts[id] = storedBlob{
		meta: ArtifactMeta{
			ArtifactID:  id,
			AccountID:   accountID,
			Size:        int64(len(buf)),
			ContentType: ct,
			StoredAt:    now,
		},
		data: buf,
	}
	m.byAccount[accountID] = append(m.byAccount[accountID], id)
	m.mu.Unlock()

	return id, nil
}

func (m *memoryStoragePort) GetArtifact(_ context.Context, artifactID string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	blob, ok := m.artifacts[artifactID]
	if !ok {
		return nil, ErrArtifactNotFound
	}
	return io.NopCloser(bytes.NewReader(blob.data)), nil
}

func (m *memoryStoragePort) DeleteArtifact(_ context.Context, artifactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	blob, ok := m.artifacts[artifactID]
	if !ok {
		return nil // idempotent
	}
	delete(m.artifacts, artifactID)

	ids := m.byAccount[blob.meta.AccountID]
	for i, id := range ids {
		if id == artifactID {
			m.byAccount[blob.meta.AccountID] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	return nil
}

func (m *memoryStoragePort) ListArtifacts(_ context.Context, accountID string) ([]ArtifactMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byAccount[accountID]
	out := make([]ArtifactMeta, 0, len(ids))
	for _, id := range ids {
		if blob, ok := m.artifacts[id]; ok {
			out = append(out, blob.meta)
		}
	}
	return out, nil
}

var _ StoragePort = (*memoryStoragePort)(nil)
