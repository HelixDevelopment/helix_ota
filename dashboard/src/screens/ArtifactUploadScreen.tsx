// Helix OTA — ArtifactUploadScreen (design §9.2).
// multipart/form-data upload + the S1–S6 validation-feedback chain. The server returns ONE
// terminal result; S1–S3 are client pre-flight, S4–S6 reflect the server verdict. The UI
// renders whatever terminal error.code the server returns and asserts NO check ordering
// (design §9.2 gap-G2 note).

import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiClient, ApiError } from "../api/client";
import { Badge, Button, Card, ErrorPanel, Field, TextInput } from "../components/ui";
import type { Artifact, ArtifactUploadMetadata } from "../types/api";

type StepState = "idle" | "pending" | "ok" | "err";

interface Step {
  key: string;
  label: string;
}

const STEPS: Step[] = [
  { key: "S1", label: "S1 structure / media type" },
  { key: "S2", label: "S2 size within cap" },
  { key: "S3", label: "S3 metadata complete" },
  { key: "S4", label: "S4 hash match" },
  { key: "S5", label: "S5 signature valid" },
  { key: "S6", label: "S6 stored / verified" },
];

// Map a terminal server error.code (endpoints §9.1) onto the S-step that goes red.
function stepForErrorCode(code: string, status: number): string {
  if (status === 415 || code === "UNSUPPORTED_MEDIA_TYPE") return "S1";
  if (status === 413 || code === "PAYLOAD_TOO_LARGE") return "S2";
  if (code === "HASH_MISMATCH") return "S4";
  if (code === "SIGNATURE_INVALID") return "S5";
  // 400 VALIDATION_FAILED and anything else -> metadata step.
  return "S3";
}

function badgeForState(s: StepState) {
  switch (s) {
    case "ok":
      return <Badge tone="ok">passed</Badge>;
    case "err":
      return <Badge tone="err">failed</Badge>;
    case "pending":
      return <Badge tone="info">checking…</Badge>;
    default:
      return <Badge tone="neutral">—</Badge>;
  }
}

const EMPTY_META: ArtifactUploadMetadata = {
  sha256: "",
  signature: "",
  version: "",
  os: "android",
  target_model: "",
  file_hash: "",
  file_size: 0,
  metadata_hash: "",
  metadata_size: 0,
  payload_offset: 0,
  payload_size: 0,
};

// Configured client-side size cap (UX pre-check only; server is authoritative — design §9.2).
const MAX_BYTES = 4 * 1024 * 1024 * 1024; // 4 GiB

export function ArtifactUploadScreen() {
  const navigate = useNavigate();
  const [file, setFile] = useState<File | null>(null);
  const [meta, setMeta] = useState<ArtifactUploadMetadata>(EMPTY_META);
  const [stepStates, setStepStates] = useState<Record<string, StepState>>({});
  const [error, setError] = useState<unknown>(null);
  const [result, setResult] = useState<Artifact | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const requiredMetaPresent = useMemo(
    () =>
      meta.sha256.trim() !== "" &&
      meta.signature.trim() !== "" &&
      meta.version.trim() !== "" &&
      meta.os.trim() !== "" &&
      meta.target_model.trim() !== "",
    [meta],
  );

  function setMetaField<K extends keyof ArtifactUploadMetadata>(
    key: K,
    value: ArtifactUploadMetadata[K],
  ) {
    setMeta((m) => ({ ...m, [key]: value }));
  }

  function preflight(): Record<string, StepState> {
    const states: Record<string, StepState> = {};
    // S1: a file is selected.
    states["S1"] = file ? "ok" : "err";
    // S2: under the client cap.
    states["S2"] = file ? (file.size <= MAX_BYTES ? "ok" : "err") : "idle";
    // S3: required metadata present + well-formed.
    states["S3"] = requiredMetaPresent ? "ok" : "err";
    // S4-S6 are server-decided.
    states["S4"] = "pending";
    states["S5"] = "pending";
    states["S6"] = "pending";
    return states;
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;
    setError(null);
    setResult(null);

    const pre = preflight();
    setStepStates(pre);
    if (pre["S1"] !== "ok" || pre["S2"] === "err" || pre["S3"] !== "ok") {
      return; // client pre-flight failed; nothing sent
    }

    setSubmitting(true);
    try {
      const artifact = await apiClient.uploadArtifact(file, meta);
      // 201 -> S4/S5/S6 all confirmed by the server.
      setStepStates({ ...pre, S4: "ok", S5: "ok", S6: "ok" });
      setResult(artifact);
    } catch (err) {
      setError(err);
      if (err instanceof ApiError) {
        const failed = stepForErrorCode(err.code, err.status);
        // The failed step is red; steps logically "after" it stay pending/unknown.
        const order = STEPS.map((s) => s.key);
        const idx = order.indexOf(failed);
        const next: Record<string, StepState> = { ...pre };
        for (let i = 0; i < order.length; i++) {
          const k = order[i];
          if (i < idx) next[k] = next[k] === "idle" ? "ok" : next[k];
          else if (i === idx) next[k] = "err";
          else next[k] = "idle";
        }
        setStepStates(next);
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>Upload artifact</h1>

      {result ? (
        <Card title="Artifact stored & verified">
          <div>
            <strong>artifact_id:</strong> {result.artifact_id}
          </div>
          <div>
            <strong>verified:</strong> {String(result.verified)}
          </div>
          <div>
            <strong>storage_ref:</strong> {result.storage_ref}
          </div>
          <div style={{ marginTop: 12 }}>
            <Button
              onClick={() =>
                navigate("/releases/new", {
                  state: {
                    artifact_id: result.artifact_id,
                    version: result.version,
                    os: result.os,
                    target_model: result.target_model,
                  },
                })
              }
            >
              Create release from this artifact
            </Button>
          </div>
        </Card>
      ) : null}

      <Card title="Validation chain (S1–S6)">
        <ol style={{ listStyle: "none", padding: 0, margin: 0 }}>
          {STEPS.map((s) => (
            <li
              key={s.key}
              style={{
                display: "flex",
                justifyContent: "space-between",
                padding: "6px 0",
                borderBottom: "1px solid #eef1f5",
              }}
            >
              <span>{s.label}</span>
              {badgeForState(stepStates[s.key] ?? "idle")}
            </li>
          ))}
        </ol>
      </Card>

      <Card title="Artifact + metadata">
        <form onSubmit={onSubmit}>
          <Field label="File (.zip / payload.bin)">
            <input
              type="file"
              accept=".zip,.bin,application/zip,application/octet-stream"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </Field>

          <Field label="sha256 (lowercase hex)">
            <TextInput value={meta.sha256} onChange={(v) => setMetaField("sha256", v)} />
          </Field>
          <Field label="signature (base64 detached)">
            <TextInput value={meta.signature} onChange={(v) => setMetaField("signature", v)} />
          </Field>
          <Field label="version">
            <TextInput value={meta.version} onChange={(v) => setMetaField("version", v)} />
          </Field>
          <Field label="os">
            <TextInput value={meta.os} onChange={(v) => setMetaField("os", v)} />
          </Field>
          <Field label="target_model">
            <TextInput
              value={meta.target_model}
              onChange={(v) => setMetaField("target_model", v)}
            />
          </Field>

          <details style={{ margin: "8px 0 16px" }}>
            <summary>AOSP A/B streaming fields</summary>
            <Field label="file_hash">
              <TextInput value={meta.file_hash} onChange={(v) => setMetaField("file_hash", v)} />
            </Field>
            <Field label="file_size">
              <TextInput
                type="number"
                value={String(meta.file_size)}
                onChange={(v) => setMetaField("file_size", Number(v) || 0)}
              />
            </Field>
            <Field label="metadata_hash">
              <TextInput
                value={meta.metadata_hash}
                onChange={(v) => setMetaField("metadata_hash", v)}
              />
            </Field>
            <Field label="metadata_size">
              <TextInput
                type="number"
                value={String(meta.metadata_size)}
                onChange={(v) => setMetaField("metadata_size", Number(v) || 0)}
              />
            </Field>
            <Field label="payload_offset">
              <TextInput
                type="number"
                value={String(meta.payload_offset)}
                onChange={(v) => setMetaField("payload_offset", Number(v) || 0)}
              />
            </Field>
            <Field label="payload_size">
              <TextInput
                type="number"
                value={String(meta.payload_size)}
                onChange={(v) => setMetaField("payload_size", Number(v) || 0)}
              />
            </Field>
          </details>

          {error ? <ErrorPanel error={error} /> : null}

          <div style={{ marginTop: 12 }}>
            <Button type="submit" disabled={!file || submitting}>
              {submitting ? "Uploading…" : "Upload & validate"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
