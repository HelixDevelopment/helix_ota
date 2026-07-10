package api

import (
	"context"
	"net/http"
	"os"
	"testing"

	otaprotocol "github.com/HelixDevelopment/ota-protocol"
)

// TestArtifactUploadServesVerifiedSignature is the §11.4.115 RED-baseline guard
// for the "verified signature != served signature" defect surfaced by the
// Wave-8 audit of submodules/ota-artifact-validator.
//
// FACT (pre-fix code path, all citations file:line):
//   - handlers_artifact.go:124 sig = resolveSignature(form, meta.Signature).
//     resolveSignature (handlers_artifact.go:274) PREFERS an uploaded `signature`
//     form-part over metadata.signature, so `sig` is the signature actually
//     VERIFIED against the payload.
//   - handlers_artifact.go:147 in.Signature = sig -> the ONLY signature the
//     validator cryptographically checks (ota-artifact-validator/pipeline.go:73
//     ValidateSignature(digest, in.PublicKey, in.Signature)).
//   - The validator NEVER cross-checks Meta.Signature against Input.Signature;
//     S6 ValidateMetadata only requires meta.signature non-empty + non-blank
//     (ota-protocol/validate.go:79-84).
//   - handlers_artifact.go:177 art.Signature = meta.Signature -> the STORED
//     value is metadata.signature, which can differ from the verified `sig`.
//   - handlers_client.go:81 UpdateAvailable.Signature = art.Signature -> that
//     stored, possibly-divergent value is SERVED to the device.
//
// Consequence: an upload where the verified detached signature (form-part) and
// metadata.signature diverge is ACCEPTED (201, Verified:true), but the device is
// served a signature that does NOT match the payload -> the device fails to
// install while the server reported the artifact valid/published. This is a
// §11.4 "validation passes but broken for the end user" defect.
//
// Invariant enforced here: the signature the server SERVES to a device MUST be
// exactly the one it VERIFIED against the payload (single source of truth).
//
// §11.4.115 polarity switch: the DEFAULT run (RED_MODE unset) is the GREEN
// regression-guard — it asserts the defect is ABSENT (served == verified), so it
// FAILS on the pre-fix artifact (that failure is the captured RED evidence) and
// PASSES on the fixed artifact. Running with RED_MODE=1 flips it to
// reproduce-and-assert the defect is PRESENT (served == divergent): it PASSES on
// the pre-fix artifact (documenting the bug) and FAILS on the fixed artifact.
func TestArtifactUploadServesVerifiedSignature(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// A device behind the target so /client/update yields a 200 update offer.
	dev := registerDevice(t, env, DeviceRegistration{
		HardwareID:     "sig-consistency-hw",
		Model:          "OrangePi5Max",
		OS:             otaprotocol.OSAndroid,
		CurrentVersion: "1.0.0",
		Group:          "field-fleet-a",
	})

	payload := []byte("signature-consistency payload bytes")
	file := zipStored(t, payload)
	digest := sha256Hex(file)

	// verifiedSig: a VALID detached signature over the real payload digest,
	// supplied as the `signature` form-part (resolveSignature prefers it), so it
	// is the signature the S3 stage actually verifies -> upload is accepted.
	verifiedSig := env.signDigest(digest)

	// divergentSig: a different but well-formed base64 ed25519 signature (over an
	// unrelated digest). It is non-empty + non-blank so S6 accepts it, but it does
	// NOT match the payload. It is placed in metadata.signature.
	divergentSig := env.signDigest(sha256Hex([]byte("unrelated content - not the payload")))
	if divergentSig == verifiedSig {
		t.Fatalf("test setup invalid: divergent and verified signatures collided")
	}

	meta := env.validMeta(file, "1.1.0")
	meta.Signature = divergentSig // stored/served field diverges from the verified one

	body, ct := uploadMultipartParts(t, file, meta, map[string][]byte{
		"signature": []byte(verifiedSig),
	})
	uw := env.do(http.MethodPost, "/api/v1/artifacts/upload", env.adminToken(), body, ct)
	if uw.Code != http.StatusCreated {
		t.Fatalf("upload want 201 Created (S3 verifies the form-part signature), got %d (%s)",
			uw.Code, uw.Body.String())
	}
	var art Artifact
	env.decode(uw, &art)
	if !art.Verified {
		t.Fatalf("artifact should be reported verified")
	}

	// Publish a release + all-targets deployment so the device is offered this
	// artifact through the real device-facing path.
	rw := env.doJSON(http.MethodPost, "/api/v1/releases", env.adminToken(), ReleaseCreate{
		ArtifactID: art.ArtifactID, Version: "1.1.0", OS: otaprotocol.OSAndroid, TargetModel: "OrangePi5Max",
	})
	if rw.Code != http.StatusCreated {
		t.Fatalf("release want 201, got %d (%s)", rw.Code, rw.Body.String())
	}
	var rel Release
	env.decode(rw, &rel)
	dw := env.doJSON(http.MethodPost, "/api/v1/deployments", env.adminToken(), DeploymentCreate{
		ReleaseID: rel.ReleaseID, Strategy: "all-targets", Group: "field-fleet-a",
	})
	if dw.Code != http.StatusCreated {
		t.Fatalf("deployment want 201, got %d (%s)", dw.Code, dw.Body.String())
	}

	// Drive the real device-facing update check (handlers_client.go) and read the
	// signature the device would actually receive.
	cw := env.do(http.MethodGet, "/api/v1/client/update", env.deviceToken(dev.DeviceID), nil, "")
	if cw.Code != http.StatusOK {
		t.Fatalf("behind device want 200 update offer, got %d (%s)", cw.Code, cw.Body.String())
	}
	var upd otaprotocol.UpdateAvailable
	env.decode(cw, &upd)

	// Secondary check: the stored artifact record (the source of the served value)
	// must also carry the verified signature.
	stored, err := env.repo.GetArtifact(ctx, art.ArtifactID)
	if err != nil {
		t.Fatalf("get stored artifact: %v", err)
	}

	if os.Getenv("RED_MODE") == "1" {
		// Reproduce-and-assert the defect is PRESENT: the served signature is the
		// divergent metadata.signature, NOT the verified one. This PASSES on the
		// pre-fix (broken) artifact, documenting the bug.
		if upd.Signature != divergentSig {
			t.Fatalf("RED_MODE: expected the device to be served the divergent metadata.signature %q, got %q",
				divergentSig, upd.Signature)
		}
		return
	}

	// GREEN regression-guard (default): the device MUST be served the signature the
	// server VERIFIED against the payload. FAILS on the pre-fix artifact (captured
	// RED evidence); PASSES on the fixed artifact.
	if upd.Signature != verifiedSig {
		t.Fatalf("device served a signature that does NOT match the verified one: "+
			"server verified %q against the payload but serves %q to the device -> the "+
			"device receives a signature that does not match the payload and fails to "+
			"install while the server reported the artifact valid/published",
			verifiedSig, upd.Signature)
	}
	if stored.Signature != verifiedSig {
		t.Fatalf("stored artifact signature %q != verified signature %q (single-source-of-truth violated)",
			stored.Signature, verifiedSig)
	}
}
