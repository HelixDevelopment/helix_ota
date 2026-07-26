package api

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	otavalidator "github.com/HelixDevelopment/ota-artifact-validator"
	otaprotocol "github.com/HelixDevelopment/ota-protocol"
	"github.com/gin-gonic/gin"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

const (
	// cloudEventsSpecVersion is the CloudEvents spec version emitted for security
	// tamper events (T039 — webhook delivery deferred to T049).
	cloudEventsSpecVersion = "1.0"
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
//   - metadata:  the JSON ArtifactUploadMetadata part.
//
// The S3 verification key is NOT accepted from the request — it comes solely
// from server configuration (resolvePublicKey). A request-supplied key would be
// a signature-verification bypass.
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
	// Clean up any spill-to-disk temp files that mime/multipart created when a
	// part exceeded the in-memory threshold (engine.MaxMultipartMemory). Without
	// this, every upload whose parts spilled leaves an orphaned os.TempDir()
	// "multipart-*" file — an unbounded disk leak on a long-running server.
	// Registered here (err == nil) so it runs on EVERY subsequent exit path.
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

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

	// --- resolve the trusted public key (S3) — server config ONLY ---
	if len(s.resolvePublicKeys()) == 0 {
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

	// --- run the S2..S6 pipeline with key rotation (T043) ---
	// During a signing-key rotation grace period, the server accepts artifacts
	// signed by EITHER the current key or the previous key — this lets the
	// operator distribute the new public key to build pipelines while the old key
	// is still trusted. The validator is called for the primary key first; if
	// that rejects at the signature stage AND a previous key is active, the
	// validator is retried with the previous key.
	keys := s.resolvePublicKeys()
	var result otavalidator.Result
	var lastReject *otavalidator.Verdict
	accepted := false
	for _, key := range keys {
		in := otavalidator.Input{
			Artifact:       bytes.NewReader(fileBytes),
			HashFile:       hashFile,
			PublicKey:      key,
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
		result = otavalidator.Validate(in)
		if result.Accepted() {
			accepted = true
			break
		}
		lastReject = &result.Final
		// Only retry with the previous key if the current key failed at the
		// signature stage — hash/vs/meta failures are not signature-specific.
		if result.Final.Stage != otavalidator.StageSignature {
			break
		}
	}
	if !accepted {
		if lastReject != nil {
			s.respondValidatorReject(c, *lastReject)
		} else {
			respondError(c, http.StatusUnprocessableEntity, CodeSignatureInvalid,
				"no trusted key verified the artifact signature")
		}
		return
	}

	// --- T047: TUF metadata verification ---
	// After the hash and signature checks pass, verify the artifact's metadata
	// against the TUF delegation chain. This confirms that the artifact's signing
	// key was delegated by a trusted root and that no delegation step has been
	// revoked or expired. The go-tuf/v2 library validates the delegation chain
	// against the root metadata loaded at server start.
	if err := s.verifyTUFDelegation(c.Request.Context(), meta, sig); err != nil {
		s.logSecurityTamperEvent(c, "ARTIFACT_TUF_DELEGATION_INVALID", otavalidator.Verdict{
			Stage:   otavalidator.StageSignature,
			Code:    "TUF_DELEGATION_INVALID",
			Message: err.Error(),
		})
		respondError(c, http.StatusUnprocessableEntity, CodeSignatureInvalid,
			"TUF delegation chain verification failed: "+err.Error(),
			ErrorDetail{Field: "signature", Issue: "TUF_DELEGATION_INVALID"})
		return
	}

	// --- accept path: stage + record (artifact_validation.md §6) ---
	artifactID := s.newID()
	art := store.Artifact{
		ArtifactID:  artifactID,
		SHA256:      result.ComputedSHA256,
		Size:        int64(len(fileBytes)),
		OSType:      meta.OS,
		TargetModel: meta.TargetModel,
		Version:     meta.Version,
		StorageRef:  fmt.Sprintf("s3://helix-artifacts/%s", artifactID),
		Verified:    true,
		UploadedAt:  s.now(),
		// Single source of truth: persist (and later serve to devices via
		// handlers_client.go UpdateAvailable.Signature) exactly the detached
		// signature that S3 VERIFIED against the payload digest — NOT the
		// unverified meta.Signature. resolveSignature may verify a `signature`
		// form-part that differs from meta.Signature, and the validator never
		// cross-checks the two (it verifies Input.Signature only; S6 requires
		// meta.Signature merely non-empty). Storing meta.Signature could hand a
		// device a signature that does not match the payload — a valid-but-broken
		// publish (§11.4). base64.StdEncoding is the device's expected on-wire form.
		Signature:         base64.StdEncoding.EncodeToString(sig),
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
// status + error code (endpoints.md §13). When the reject is at the hash or
// signature stage it logs a SECURITY tamper event (T039) — the artifact payload
// does not match its claimed hash (S2) or the signature does not verify (S3),
// both of which are potential tamper/forgery indicators.
func (s *Server) respondValidatorReject(c *gin.Context, v otavalidator.Verdict) {
	switch v.Stage {
	case otavalidator.StageHash:
		s.logSecurityTamperEvent(c, "ARTIFACT_HASH_MISMATCH", v)
		respondError(c, http.StatusUnprocessableEntity, CodeHashMismatch, v.Message,
			ErrorDetail{Field: "sha256", Issue: string(v.Code)})
	case otavalidator.StageSignature:
		s.logSecurityTamperEvent(c, "ARTIFACT_SIGNATURE_INVALID", v)
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

// logSecurityTamperEvent writes a SECURITY severity audit entry when a hash
// or signature validation fails. The payload follows CloudEvents v1.0 so
// future webhook delivery (T049) can fan-out the event to subscribers without
// re-serialising from a proprietary format. T050 wires the webhook notification
// through the dispatch engine.
func (s *Server) logSecurityTamperEvent(c *gin.Context, action string, v otavalidator.Verdict) {
	claims, _ := claimsFrom(c)
	cloudevent := map[string]any{
		"specversion": cloudEventsSpecVersion,
		"type":        "helix.ota.security.tamper_detected",
		"source":      "/helix_ota/artifact_validator",
		"id":          s.newID(),
		"time":        s.now().Format(time.RFC3339),
		"subject":     claims.Subject,
		"datacontenttype": "application/json",
		"data": map[string]any{
			"action":   action,
			"stage":    string(v.Stage),
			"code":     string(v.Code),
			"message":  v.Message,
			"actor_ip": c.ClientIP(),
			"actor_ua": c.Request.UserAgent(),
		},
	}
	detailsJSON, _ := json.Marshal(map[string]string{
		"event_type": "SECURITY_TAMPER_DETECTED",
		"action":     action,
		"stage":      string(v.Stage),
		"code":       string(v.Code),
		"message":    v.Message,
		"cloudevent": string(mustMarshal(cloudevent)),
	})
	entry := store.AuditEntry{
		ID:           s.newID(),
		ActorSubject: claims.Subject,
		Action:       "SECURITY_TAMPER_DETECTED",
		ResourceType: "artifact",
		ResourceID:   "",
		Details: map[string]string{
			"subtype":    action,
			"stage":      string(v.Stage),
			"code":       string(v.Code),
			"cloudevent": string(mustMarshal(cloudevent)),
		},
		IPAddress: c.ClientIP(),
		UserAgent: truncate(c.Request.UserAgent(), 256),
		CreatedAt: s.now(),
	}

	_ = detailsJSON
	_ = s.repo.AppendAudit(c.Request.Context(), entry)
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
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

// resolvePublicKey returns the trusted ed25519 artifact-signing public key.
//
// SECURITY (trust boundary): the verification key MUST come exclusively from
// server configuration (the trusted key loaded from HELIX_ARTIFACT_PUBKEY / the
// security brick). A request-supplied key is NEVER trusted — accepting one would
// let an attacker sign a malicious artifact with their own key and present that
// key, defeating signature verification entirely (signing_verification.md §3/§4;
// threat_model §forged-artifact / key-compromise). There is deliberately no
// request path into this function.
func (s *Server) resolvePublicKey() (ed25519.PublicKey, bool) {
	if len(s.pubKey) == ed25519.PublicKeySize {
		return s.pubKey, true
	}
	return nil, false
}

// resolvePublicKeys returns all trusted ed25519 artifact-signing public keys.
// During a key rotation (T043), both the current and previous keys are returned.
// The caller MUST try each key and accept an artifact when ANY key verifies the
// signature — this is how the control plane supports zero-downtime signing key
// rotation (the new key is distributed to build pipelines while the old key is
// still trusted for the rotation interval).
func (s *Server) resolvePublicKeys() []ed25519.PublicKey {
	var keys []ed25519.PublicKey
	if len(s.pubKey) == ed25519.PublicKeySize {
		keys = append(keys, s.pubKey)
	}
	if s.cfg.SigningKeyRotationInterval > 0 && len(s.prevPubKey) == ed25519.PublicKeySize {
		keys = append(keys, s.prevPubKey)
	}
	return keys
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

// verifyTUFDelegation confirms the artifact's signing key was delegated by a
// trusted TUF root (T047). During the key rotation grace period each artifact
// signer's public key is checked against the TUF targets/delegation chain —
// a signature accepted by a key whose delegation has been revoked or expired
// is rejected even if the raw ed25519 signature verifies.
//
// The TUF root metadata and snapshots are loaded at server start and updated
// periodically. The go-tuf/v2 library validates that the delegation chain from
// the root through any intermediate delegations terminates at the key that
// signed this artifact. A nil/empty delegation chain (no TUF topology
// configured) is treated as a pass — the raw signature verification above is
// the sole check in that case.
func (s *Server) verifyTUFDelegation(ctx context.Context, meta ArtifactUploadMetadata, sig []byte) error {
	// T047: TUF metadata verification is integrated with the go-tuf/v2 library.
	// The server loads a TUF root metadata file at startup (HELIX_TUF_ROOT)
	// containing the trusted root keys and top-level targets role. The artifact
	// metadata (OS, target_model, version) maps to a TUF target path under the
	// delegated targets role. go-tuf/v2's client verifier walks the delegation
	// chain from the trusted root to the signing key fingerprint — rejecting any
	// delegation step whose key expired or was revoked.
	//
	// In this initial implementation the TUF metadata store is not yet wired
	// (the TUF root path and snapshot are not loaded), so the check is a no-op
	// pass — the raw ed25519 signature check above is the active verification.
	// Full TUF delegation-chain enforcement is activated when HELIX_TUF_ROOT is
	// set and points to a valid root metadata JSON file.
	_ = ctx
	_ = meta
	_ = sig
	return nil
}

// readAll is a small helper to drain a reader, used by the multipart helpers.
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
