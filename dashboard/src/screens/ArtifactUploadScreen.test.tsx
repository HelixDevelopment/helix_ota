// Helix OTA dashboard — component test for the ArtifactUploadScreen (design §9.2).
// Drives the S1–S6 validation-feedback chain + the multipart upload submit through
// the screen's real client logic, stubbing only the apiClient.uploadArtifact call
// (unit/component-test mock, permitted §11.4.27 — every assertion exercises real
// user-visible behaviour: file selection, the client pre-flight gate, the per-step
// badges, the success card, and the terminal-error → S-step mapping).

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { ApiError } from "../api/client";
import type { Artifact, ArtifactUploadMetadata } from "../types/api";

// Mock the singleton client the screen imports. Only uploadArtifact is exercised.
const uploadArtifact = vi.fn<[File, ArtifactUploadMetadata], Promise<Artifact>>();
vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual, // keep the real ApiError class so `instanceof` works in the screen
    apiClient: {
      uploadArtifact: (...args: unknown[]) =>
        uploadArtifact(...(args as [File, ArtifactUploadMetadata])),
    },
  };
});

// Import AFTER the mock is registered.
import { ArtifactUploadScreen } from "./ArtifactUploadScreen";

function renderScreen() {
  return render(
    <MemoryRouter>
      <ArtifactUploadScreen />
    </MemoryRouter>,
  );
}

// The S1–S6 chain is rendered as an <ol> of <li> rows; each row shows a label and a
// status Badge ("passed"/"failed"/"checking…"/"—"). Return the row text for a step key.
function stepRow(key: string): HTMLElement {
  const label = screen.getByText(new RegExp(`^${key} `));
  // The <li> is the label's parent flex row.
  return label.closest("li") as HTMLElement;
}

const fileInput = () => document.querySelector('input[type="file"]') as HTMLInputElement;

// Fill the five required metadata text fields (Field wraps <span>label</span> + input).
function fillRequiredMeta() {
  fireEvent.change(screen.getByText("sha256 (lowercase hex)").parentElement!.querySelector("input")!, {
    target: { value: "a".repeat(64) },
  });
  fireEvent.change(
    screen.getByText("signature (base64 detached)").parentElement!.querySelector("input")!,
    { target: { value: "c2lnbmF0dXJl" } },
  );
  fireEvent.change(screen.getByText("version").parentElement!.querySelector("input")!, {
    target: { value: "1.0.0" },
  });
  // os is pre-filled to "android"; target_model is required + empty.
  fireEvent.change(screen.getByText("target_model").parentElement!.querySelector("input")!, {
    target: { value: "OrangePi5Max" },
  });
}

function selectFile(name = "ota.zip", bytes = new Uint8Array([0x50, 0x4b, 0x05, 0x06])) {
  const f = new File([bytes], name, { type: "application/zip" });
  fireEvent.change(fileInput(), { target: { files: [f] } });
  return f;
}

describe("ArtifactUploadScreen", () => {
  beforeEach(() => {
    uploadArtifact.mockReset();
  });

  it("renders the upload form, the S1–S6 chain, and a disabled submit before a file is picked", () => {
    renderScreen();
    expect(screen.getByRole("heading", { name: "Upload artifact", level: 1 })).toBeInTheDocument();
    // The whole S1–S6 chain is present.
    for (const k of ["S1", "S2", "S3", "S4", "S5", "S6"]) {
      expect(stepRow(k)).toBeInTheDocument();
    }
    // The file <input> exists; submit is disabled with no file selected.
    expect(fileInput()).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload & validate" })).toBeDisabled();
  });

  it("enables submit once a file is selected (file-select interaction)", async () => {
    renderScreen();
    const submit = screen.getByRole("button", { name: "Upload & validate" });
    expect(submit).toBeDisabled();
    selectFile();
    await waitFor(() => expect(submit).toBeEnabled());
  });

  it("blocks the upload at the client pre-flight when required metadata is missing (S3 → failed)", async () => {
    renderScreen();
    selectFile();
    // No metadata filled -> S3 must fail the client pre-flight and NOTHING is sent.
    fireEvent.click(screen.getByRole("button", { name: "Upload & validate" }));

    await waitFor(() => expect(within(stepRow("S1")).getByText("passed")).toBeInTheDocument());
    expect(within(stepRow("S3")).getByText("failed")).toBeInTheDocument();
    // Server stages stay "checking…" (never reached); the client never called the API.
    expect(within(stepRow("S4")).getByText("checking…")).toBeInTheDocument();
    expect(uploadArtifact).not.toHaveBeenCalled();
  });

  it("submits via uploadArtifact and renders the stored+verified success card (S4/S5/S6 → passed)", async () => {
    const stored: Artifact = {
      artifact_id: "art_seed_1",
      sha256: "a".repeat(64),
      os: "android",
      target_model: "OrangePi5Max",
      version: "1.0.0",
      storage_ref: "s3://helix-artifacts/art_seed_1",
      verified: true,
      created_at: "2026-06-10T00:00:00Z",
    };
    uploadArtifact.mockResolvedValue(stored);

    renderScreen();
    const file = selectFile();
    fillRequiredMeta();
    fireEvent.click(screen.getByRole("button", { name: "Upload & validate" }));

    // The success card renders with the server-returned artifact fields.
    await waitFor(() =>
      expect(screen.getByText("Artifact stored & verified")).toBeInTheDocument(),
    );
    expect(screen.getByText("art_seed_1")).toBeInTheDocument();
    expect(screen.getByText("s3://helix-artifacts/art_seed_1")).toBeInTheDocument();
    // All six steps are "passed" on a 201.
    for (const k of ["S1", "S2", "S3", "S4", "S5", "S6"]) {
      expect(within(stepRow(k)).getByText("passed")).toBeInTheDocument();
    }
    // uploadArtifact was called with the selected File + the assembled metadata.
    expect(uploadArtifact).toHaveBeenCalledTimes(1);
    const [sentFile, sentMeta] = uploadArtifact.mock.calls[0];
    expect(sentFile).toBe(file);
    expect(sentMeta).toMatchObject({
      sha256: "a".repeat(64),
      signature: "c2lnbmF0dXJl",
      version: "1.0.0",
      os: "android",
      target_model: "OrangePi5Max",
    });
    // The "Create release from this artifact" CTA is offered.
    expect(
      screen.getByRole("button", { name: "Create release from this artifact" }),
    ).toBeInTheDocument();
  });

  it("maps a 422 SIGNATURE_INVALID server reject onto the S5 step and surfaces the error panel", async () => {
    uploadArtifact.mockRejectedValue(
      new ApiError(422, "SIGNATURE_INVALID", "signature does not verify", "req-sig"),
    );
    renderScreen();
    selectFile();
    fillRequiredMeta();
    fireEvent.click(screen.getByRole("button", { name: "Upload & validate" }));

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("422 SIGNATURE_INVALID");
      expect(alert).toHaveTextContent("signature does not verify");
    });
    // S5 is the failed step; S1–S3 (before it) read passed, S6 (after) resets to "—".
    expect(within(stepRow("S5")).getByText("failed")).toBeInTheDocument();
    expect(within(stepRow("S1")).getByText("passed")).toBeInTheDocument();
    expect(within(stepRow("S6")).getByText("—")).toBeInTheDocument();
  });

  it("maps a 415 UNSUPPORTED_MEDIA_TYPE reject onto the S1 step", async () => {
    uploadArtifact.mockRejectedValue(
      new ApiError(415, "UNSUPPORTED_MEDIA_TYPE", "not a zip", "req-415"),
    );
    renderScreen();
    selectFile();
    fillRequiredMeta();
    fireEvent.click(screen.getByRole("button", { name: "Upload & validate" }));

    await waitFor(() =>
      expect(within(stepRow("S1")).getByText("failed")).toBeInTheDocument(),
    );
    expect(screen.getByRole("alert")).toHaveTextContent("415 UNSUPPORTED_MEDIA_TYPE");
  });

  it("shows the transient 'Uploading…' label while the upload is in flight", async () => {
    let resolveUpload!: (a: Artifact) => void;
    uploadArtifact.mockReturnValue(
      new Promise<Artifact>((res) => {
        resolveUpload = res;
      }),
    );
    renderScreen();
    selectFile();
    fillRequiredMeta();
    fireEvent.click(screen.getByRole("button", { name: "Upload & validate" }));

    // The button flips to its in-flight label and is disabled while submitting.
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Uploading…" })).toBeDisabled(),
    );
    // Let it settle so the test leaves no pending state.
    resolveUpload({
      artifact_id: "art_x",
      sha256: "a".repeat(64),
      os: "android",
      target_model: "OrangePi5Max",
      version: "1.0.0",
      storage_ref: "s3://helix-artifacts/art_x",
      verified: true,
      created_at: "2026-06-10T00:00:00Z",
    });
    await waitFor(() =>
      expect(screen.getByText("Artifact stored & verified")).toBeInTheDocument(),
    );
  });
});
