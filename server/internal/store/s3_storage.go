package store

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3StoragePort implements StoragePort backed by MinIO or any S3-compatible
// object store. Artifacts are stored under the key prefix
// {accountID}/{artifactID} so ListArtifacts can efficiently enumerate
// one account's blobs.
type s3StoragePort struct {
	client   *minio.Client
	bucket   string
	maxBytes int64
}

// S3StorageConfig holds the parameters needed to dial an S3-compatible store.
type S3StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

// NewS3StoragePort dials the configured S3 endpoint and returns a StoragePort.
func NewS3StoragePort(cfg S3StorageConfig) (StoragePort, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("store: S3 endpoint is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("store: S3 access key and secret key are required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("store: S3 bucket is required")
	}

	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("store: minio dial: %w", err)
	}

	return &s3StoragePort{
		client:   cli,
		bucket:   cfg.Bucket,
		maxBytes: DefaultMaxArtifactBytes,
	}, nil
}

func (s *s3StoragePort) objectKey(accountID, artifactID string) string {
	return fmt.Sprintf("%s/%s", accountID, artifactID)
}

func (s *s3StoragePort) StoreArtifact(ctx context.Context, accountID string, data io.Reader, size int64) (string, error) {
	if accountID == "" {
		return "", ErrStorageBackend
	}
	if size > s.maxBytes {
		return "", ErrArtifactTooLarge
	}

	// Read up to 512 bytes for content-type validation, then reconstruct the
	// reader with a MultiReader so the server never consumes bytes the S3
	// layer needs.
	head := make([]byte, 512)
	n, readErr := io.ReadFull(data, head)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return "", ErrStorageBackend
	}
	head = head[:n]

	if isRejectedContentType(head) {
		return "", ErrInvalidContentType
	}

	ct := detectContentType(head)
	id := genStorageID()
	key := s.objectKey(accountID, id)

	combined := io.MultiReader(
		strings.NewReader(string(head)),
		data,
	)

	_, err := s.client.PutObject(ctx, s.bucket, key, combined, size, minio.PutObjectOptions{
		ContentType: ct,
	})
	if err != nil {
		return "", fmt.Errorf("store: s3 put %s/%s: %w", s.bucket, key, err)
	}

	return id, nil
}

func (s *s3StoragePort) GetArtifact(ctx context.Context, artifactID string) (io.ReadCloser, error) {
	if artifactID == "" {
		return nil, ErrArtifactNotFound
	}

	// Scan all account prefixes — ListObjects with a suffix wildcard is not
	// natively supported by S3 in a single call, so we probe common prefixes.
	// In practice the caller should know the accountID; this is a best-effort
	// lookup for the case where only the artifactID is available.
	obj, found := s.findObject(ctx, artifactID)
	if !found {
		return nil, ErrArtifactNotFound
	}

	rc, err := s.client.GetObject(ctx, s.bucket, obj.Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("store: s3 get %s/%s: %w", s.bucket, obj.Key, err)
	}
	return rc, nil
}

func (s *s3StoragePort) DeleteArtifact(ctx context.Context, artifactID string) error {
	obj, found := s.findObject(ctx, artifactID)
	if !found {
		return nil // idempotent
	}

	err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("store: s3 delete %s/%s: %w", s.bucket, obj.Key, err)
	}
	return nil
}

func (s *s3StoragePort) ListArtifacts(ctx context.Context, accountID string) ([]ArtifactMeta, error) {
	prefix := accountID + "/"
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})

	var out []ArtifactMeta
	for obj := range ch {
		if obj.Err != nil {
			return nil, fmt.Errorf("store: s3 list %s/%s: %w", s.bucket, prefix, obj.Err)
		}
		artifactID := strings.TrimPrefix(obj.Key, prefix)
		if artifactID == "" || strings.Contains(artifactID, "/") {
			continue // skip directory markers and nested keys
		}
		ct := ""
		if h, err := s.client.StatObject(ctx, s.bucket, obj.Key, minio.StatObjectOptions{}); err == nil {
			ct = h.ContentType
		}
		out = append(out, ArtifactMeta{
			ArtifactID:  artifactID,
			AccountID:   accountID,
			Size:        obj.Size,
			ContentType: ct,
			StoredAt:    obj.LastModified,
		})
	}
	return out, nil
}

// findObject does a best-effort lookup of an artifact by id when the account is
// unknown. It lists all objects and matches by suffix. In production the caller
// should know and supply the accountID — this is a fallback for the
// artifactID-only GetArtifact/DeleteArtifact paths.
func (s *s3StoragePort) findObject(ctx context.Context, artifactID string) (minio.ObjectInfo, bool) {
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Recursive: true,
	})
	for obj := range ch {
		if obj.Err != nil {
			continue
		}
		if strings.HasSuffix(obj.Key, "/"+artifactID) || obj.Key == artifactID {
			return obj, true
		}
	}
	return minio.ObjectInfo{}, false
}

var _ StoragePort = (*s3StoragePort)(nil)
