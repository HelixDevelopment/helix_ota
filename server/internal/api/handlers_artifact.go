package api

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// handleUploadArtifact ingests a multipart OTA artifact and runs the
// server-side validation pipeline (endpoints.md §9.1; artifact_validation.md).
//
// Multipart parts:
//   - file:      the OTA package bytes (the .zip / payload). S1 structure check
//     runs here; S2..S6 run in ota-artifact-validator.Validate.
//   - .sha256:   the mandatory external hash file (S2). Accepted as a part named
//     "sha256"; if absent, the metadata.sha256 value is used to
//     synthesize the hash-file content so the pipeline still runs S2.
//   - signature: optional raw detached signature part; if absent the base64
//     metadata.signature is used (S3).
//   - pubkey:    optional ed25519 public-key part (raw or base64); if absent the
//     server's configured trusted key is used (S3).
//   - metadata:  the JSON ArtifactUploadMetadata part.
//
// Reject codes map per artifact_validation.md §5 / endpoints.md §13:
//
//	S1 (structure)            -> 400 VALIDATION_FAILED (or 415 for non-zip)
//	S2 (hash)                 -> 422 HASH_MISMATCH
//	S3 (signature)            -> 422 SIGNATURE_INVALID
//	S4 (version monotonicity) -> 409 VERSION_NOT_MONOTONIC
//	S5 (target compatibility) -> 400 VALIDATION_FAILED
//	S6 (metadata)             -> 400 VALIDATION_FAILED
func (s *Server) handleUploadArtifact(c *gin.Context) {
	// Enforce the configured upload-size cap (endpoints.md §9.1 413).
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.MaxUploadBytes)

	if !strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		respondError(c, http.StatusUnsupportedMediaType, CodeUnsupportedMedia,
			"artifact upload must be multipart/form-data")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		if isTooLarge(err) {
			respondError(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge,
				"artifact exceeds the configured size cap")
			return
		}
		respondValidation(c, "could not parse multipart upload")
		return
	}

	// --- read the file part ---
	fileBytes, ok := readFilePart(form, "file")
	if !ok {
		respondValidation(c, "multipart part 'file' is required",
			ErrorDetail{Field: "file", Issue: "required"})
		return
	}

	// --- read the metadata part ---
	metaRaw, ok := readValuePart(form, "metadata")
	if !ok {
		respondValidation(c, "multipart part 'metadata' is required",
			ErrorDetail{Field: "metadata", Issue: "required"})
		return
	}
	var meta ArtifactUploadMetadata
	dec := json.NewDecoder(bytes.NewReader([]byte(metaRaw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&meta); err != nil {
		respondValidation(c, "metadata part is not valid ArtifactUploadMetadata JSON",
			ErrorDetail{Field: "metadata", Issue: err.Error()})
		return
	}
	if meta.SHA256 == "" || meta.Signature == "" || meta.Version == "" || meta.OS == "" || meta.TargetModel == "" {
		respondValidation(c, "metadata is missing required fields",
			ErrorDetail{Field: "sha256", Issue: "required"},
			ErrorDetail{Field: "signature", Issue: "required"},
			ErrorDetail{Field: "version", Issue: "required"},
			ErrorDetail{Field: "os", Issue: "required"},
			ErrorDetail{Field: "target_model", Issue: "required"})
		return
	}

	// --- S1: structure (ZIP_STORED) — performed by the upload handler before
	// calling the S2..S6 library (artifact_validation.md §3, §5.1) ---
	if verdict := validateStructure(fileBytes); verdict.IsReject() {
		status := http.StatusBadRequest
		code := CodeValidationFailed
		if verdict.Code == "S1_NOT_ZIP" {
			status = http.StatusUnsupportedMediaType
			code = CodeUnsupportedMedia
		}
		respondError(c, status, code, verdict.Message,
			ErrorDetail{Field: "file", Issue: string(verdict.Code)})
		return
	}

	// --- resolve the trusted public key (S3) ---
	pubKey, ok := s.resolvePublicKey(form)
	if !ok {
		respondError(c, http.StatusUnprocessableEntity, CodeSignatureInvalid,
			"no trusted signing key configured to verify the artifact signature")
		return
	}

	// --- resolve the detached signature (S3) ---
	sig, ok := resolveSignature(form, meta.Signature)
	if !ok {
		respondError(c, http.StatusUnprocessableEntity, CodeSignatureInvalid,
			"signature is missing or not valid base64",
			ErrorDetail{Field: "signature", Issue: "must be a base64 detached signature"})
		return
	}

	// --- resolve the hash file (S2): prefer an uploaded .sha256 part, else
	// synthesize from metadata.sha256 ---
	hashFile := resolveHashFile(form, meta.SHA256)

	// --- prior version for S4 monotonicity ---
	current := ""
	if latest, lerr := s.repo.LatestRelease(c.Request.Context(), meta.OS, meta.TargetModel); lerr == nil {
		current = latest.Version
	}

	// --- run the S2..S6 pipeline ---
	in := otavalidator.Input{
		Artifact:       bytes.NewReader(fileBytes),
		HashFile:       hashFile,
		PublicKey:      pubKey,
		Signature:      sig,
		CurrentVersion: current,
		Meta: otaprotocol.ArtifactMeta{
			SHA256:    meta.SHA256,
			Size:      int64(len(fileBytes)),
			OSType:    meta.OS,
			Board:     meta.TargetModel,
			Version:   meta.Version,
			Signature: meta.Signature,
		},
		TargetPolicy: s.target,
	}
	result := otavalidator.Validate(in)
	if !result.Accepted() {
		s.respondValidatorReject(c, result.Final)
		return
	}

	// --- accept path: stage + record (artifact_validation.md §6) ---
	artifactID := s.newID()
	art := store.Artifact{
		ArtifactID:        artifactID,
		SHA256:            result.ComputedSHA256,
		Size:              int64(len(fileBytes)),
		OSType:            meta.OS,
		TargetModel:       meta.TargetModel,
		Version:           meta.Version,
		StorageRef:        fmt.Sprintf("s3://helix-artifacts/%s", artifactID),
		Verified:          true,
		UploadedAt:        s.now(),
		Signature:         meta.Signature,
		PayloadProperties: payloadPropsFromMeta(meta),
	}
	if err := s.repo.CreateArtifact(c.Request.Context(), art); err != nil {
		respondError(c, http.StatusInternalServerError, CodeInternal, "could not store artifact")
		return
	}

	c.JSON(http.StatusCreated, toArtifact(art))
}

// handleGetArtifact returns artifact metadata (not the bytes) (endpoints.md
// §9.2).
func (s *Server) handleGetArtifact(c *gin.Context) {
	art, err := s.repo.GetArtifact(c.Request.Context(), c.Param("artifactId"))
	if err != nil {
		respondError(c, http.StatusNotFound, CodeNotFound, "artifact not found")
		return
	}
	c.JSON(http.StatusOK, toArtifact(art))
}

// respondValidatorReject maps a pipeline reject verdict to the proper HTTP
// status + error code (endpoints.md §13).
func (s *Server) respondValidatorReject(c *gin.Context, v otavalidator.Verdict) {
	switch v.Stage {
	case otavalidator.StageHash:
		respondError(c, http.StatusUnprocessableEntity, CodeHashMismatch, v.Message,
			ErrorDetail{Field: "sha256", Issue: string(v.Code)})
	case otavalidator.StageSignature:
		respondError(c, http.StatusUnprocessableEntity, CodeSignatureInvalid, v.Message,
			ErrorDetail{Field: "signature", Issue: string(v.Code)})
	case otavalidator.StageVersion:
		respondError(c, http.StatusConflict, CodeVersionNotMonotonic, v.Message,
			ErrorDetail{Field: "version", Issue: string(v.Code)})
	case otavalidator.StageTarget:
		respondValidation(c, v.Message, ErrorDetail{Field: "target_model", Issue: string(v.Code)})
	case otavalidator.StageMetadata:
		respondValidation(c, v.Message, ErrorDetail{Field: "metadata", Issue: string(v.Code)})
	default:
		respondValidation(c, v.Message)
	}
}

// structureVerdict is a tiny S1 verdict (the validator library starts at S2).
type structureVerdict struct {
	Code    string
	Message string
}

// IsReject reports whether the S1 structure check rejected the upload.
func (v structureVerdict) IsReject() bool { return v.Code != "" }

// validateStructure performs S1: the upload must be a readable ZIP whose
// payload entries are stored uncompressed (ZIP_STORED) so update_engine can
// range-fetch payload.bin (artifact_validation.md §5.1). A non-ZIP body is
// reported distinctly so the handler can map it to 415.
func validateStructure(file []byte) structureVerdict {
	zr, err := zip.NewReader(bytes.NewReader(file), int64(len(file)))
	if err != nil {
		return structureVerdict{Code: "S1_NOT_ZIP", Message: "artifact is not a readable ZIP archive"}
	}
	if len(zr.File) == 0 {
		return structureVerdict{Code: "S1_MISSING_ENTRY", Message: "OTA archive contains no entries"}
	}
	for _, f := range zr.File {
		// payload.bin and the OTA ZIP entries the device range-fetches must be
		// ZIP_STORED (uncompressed). We enforce STORED for the payload entry.
		if f.Name == "payload.bin" && f.Method != zip.Store {
			return structureVerdict{
				Code:    "S1_NOT_ZIP_STORED",
				Message: "payload.bin must be stored uncompressed (ZIP_STORED)",
			}
		}
	}
	return structureVerdict{}
}

// resolvePublicKey returns the trusted ed25519 public key: an uploaded pubkey
// part (raw or base64) takes precedence, else the server's configured key.
func (s *Server) resolvePublicKey(form *multipartForm) (ed25519.PublicKey, bool) {
	if raw, ok := readFilePart(form, "pubkey"); ok {
		if key, kok := parseEd25519PublicKey(raw); kok {
			return key, true
		}
	}
	if val, ok := readValuePart(form, "pubkey"); ok {
		if key, kok := parseEd25519PublicKey([]byte(val)); kok {
			return key, true
		}
	}
	if len(s.pubKey) == ed25519.PublicKeySize {
		return s.pubKey, true
	}
	return nil, false
}

// resolveSignature returns the raw detached signature bytes: an uploaded
// signature part (raw or base64) takes precedence, else the base64 from
// metadata.signature.
func resolveSignature(form *multipartForm, metaSig string) ([]byte, bool) {
	if raw, ok := readFilePart(form, "signature"); ok {
		if sig := decodeMaybeBase64(raw); len(sig) > 0 {
			return sig, true
		}
	}
	if val, ok := readValuePart(form, "signature"); ok {
		if sig := decodeMaybeBase64([]byte(val)); len(sig) > 0 {
			return sig, true
		}
	}
	if metaSig != "" {
		if sig, err := base64.StdEncoding.DecodeString(metaSig); err == nil && len(sig) > 0 {
			return sig, true
		}
	}
	return nil, false
}

// resolveHashFile returns the S2 hash-file content: an uploaded .sha256 part
// takes precedence, else the metadata.sha256 digest is used directly (the
// pipeline tolerates a bare digest).
func resolveHashFile(form *multipartForm, metaSHA string) string {
	if raw, ok := readFilePart(form, "sha256"); ok {
		return string(raw)
	}
	if val, ok := readValuePart(form, "sha256"); ok {
		return val
	}
	return metaSHA
}

// parseEd25519PublicKey parses a raw or base64-encoded ed25519 public key.
func parseEd25519PublicKey(b []byte) (ed25519.PublicKey, bool) {
	if len(b) == ed25519.PublicKeySize {
		return ed25519.PublicKey(append([]byte(nil), b...)), true
	}
	if decoded := decodeMaybeBase64(b); len(decoded) == ed25519.PublicKeySize {
		return ed25519.PublicKey(decoded), true
	}
	return nil, false
}

// decodeMaybeBase64 returns the base64-decoded bytes if the input is valid
// base64, otherwise the raw bytes unchanged. It trims surrounding whitespace.
func decodeMaybeBase64(b []byte) []byte {
	trimmed := bytes.TrimSpace(b)
	if decoded, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil {
		return decoded
	}
	return trimmed
}

// payloadPropsFromMeta builds the typed PayloadProperties from upload metadata.
func payloadPropsFromMeta(meta ArtifactUploadMetadata) otaprotocol.PayloadProperties {
	return otaprotocol.PayloadProperties{
		FileHash:     meta.FileHash,
		FileSize:     meta.FileSize,
		MetadataHash: meta.MetadataHash,
		MetadataSize: meta.MetadataSize,
	}
}

// toArtifact maps a stored artifact to the Artifact response body.
func toArtifact(a store.Artifact) Artifact {
	return Artifact{
		ArtifactID:  a.ArtifactID,
		SHA256:      a.SHA256,
		Size:        a.Size,
		OS:          a.OSType,
		TargetModel: a.TargetModel,
		Version:     a.Version,
		StorageRef:  a.StorageRef,
		Verified:    a.Verified,
		UploadedAt:  a.UploadedAt,
	}
}

// isTooLarge reports whether err is the MaxBytesReader "request body too large"
// error.
func isTooLarge(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return strings.Contains(err.Error(), "request body too large")
}

// readAll is a small helper to drain a reader, used by the multipart helpers.
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
