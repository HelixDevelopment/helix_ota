// Helix OTA — thin, self-contained UI primitives.
// The design names an UNVERIFIED `UI-Components-React` catalogue brick (design §5/§13);
// it is NOT a confirmed dependency, so these are minimal Helix-local primitives that carry
// NO API knowledge (decoupling, design §11.4.28). Swap to the brick if/when verified (§13.1).

import type { CSSProperties, ReactNode } from "react";
import { ApiError } from "../api/client";

export function Card({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <section style={styles.card}>
      {title ? <h2 style={styles.cardTitle}>{title}</h2> : null}
      {children}
    </section>
  );
}

export function Button({
  children,
  onClick,
  disabled,
  type = "button",
  variant = "primary",
}: {
  children: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  type?: "button" | "submit";
  variant?: "primary" | "secondary";
}) {
  const base = variant === "primary" ? styles.btnPrimary : styles.btnSecondary;
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      style={{ ...styles.btn, ...base, ...(disabled ? styles.btnDisabled : null) }}
    >
      {children}
    </button>
  );
}

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label style={styles.field}>
      <span style={styles.fieldLabel}>{label}</span>
      {children}
    </label>
  );
}

export function TextInput({
  value,
  onChange,
  type = "text",
  placeholder,
  autoComplete,
}: {
  value: string;
  onChange: (v: string) => void;
  type?: "text" | "password" | "number";
  placeholder?: string;
  autoComplete?: string;
}) {
  return (
    <input
      style={styles.input}
      type={type}
      value={value}
      placeholder={placeholder}
      autoComplete={autoComplete}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

export type BadgeTone = "neutral" | "ok" | "warn" | "err" | "info";

export function Badge({ tone = "neutral", children }: { tone?: BadgeTone; children: ReactNode }) {
  return <span style={{ ...styles.badge, ...badgeTone[tone] }}>{children}</span>;
}

export function ProgressBar({ value, max = 100 }: { value: number; max?: number }) {
  const pct = max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0;
  return (
    <div style={styles.progressTrack} role="progressbar" aria-valuenow={Math.round(pct)}>
      <div style={{ ...styles.progressFill, width: `${pct}%` }} />
    </div>
  );
}

export function Table({ head, children }: { head: string[]; children: ReactNode }) {
  return (
    <table style={styles.table}>
      <thead>
        <tr>
          {head.map((h) => (
            <th key={h} style={styles.th}>
              {h}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}

// Renders an ApiError (or any error) with the stable code + request_id + field details
// (design §8: never surfaces stack traces; echoes X-Request-Id for correlation).
export function ErrorPanel({ error }: { error: unknown }) {
  if (error instanceof ApiError) {
    return (
      <div style={styles.errorPanel} role="alert">
        <strong>
          {error.status} {error.code}
        </strong>
        <div>{error.message}</div>
        {error.details.length > 0 ? (
          <ul style={styles.errorList}>
            {error.details.map((d, i) => (
              <li key={i}>{d.field ? `${d.field}: ${d.message}` : d.message}</li>
            ))}
          </ul>
        ) : null}
        {error.requestId ? (
          <div style={styles.requestId}>request_id: {error.requestId}</div>
        ) : null}
      </div>
    );
  }
  const message = error instanceof Error ? error.message : String(error);
  return (
    <div style={styles.errorPanel} role="alert">
      {message}
    </div>
  );
}

export function EmptyState({ children }: { children: ReactNode }) {
  return <div style={styles.empty}>{children}</div>;
}

// OpenDesign token mapping (design-systems/helix-ota/tokens.css, vendored to
// src/styles/tokens.css). Each inline hex is repointed to the semantic
// var(--token) per docs/research/opendesign_vendoring_plan_20260709 §2.3 step 3,
// so light/dark flips automatically via <html data-theme>.
const styles: Record<string, CSSProperties> = {
  card: {
    background: "var(--surface)", // #fff
    border: "1px solid var(--border)", // #e2e5ea
    borderRadius: "var(--radius-md)", // 8 → 12px (DESIGN.md cards)
    padding: 20,
    marginBottom: 16,
  },
  cardTitle: { margin: "0 0 12px", fontSize: 18, color: "var(--fg)" },
  btn: {
    borderRadius: "var(--radius-sm)", // 6 → 8px
    border: "1px solid transparent",
    padding: "8px 14px",
    fontSize: 14,
    cursor: "pointer",
  },
  btnPrimary: { background: "var(--accent)", color: "var(--accent-on)" }, // #2563eb / #fff
  btnSecondary: {
    background: "var(--surface)", // #fff
    color: "var(--fg)", // #1f2937
    borderColor: "var(--border)", // #cbd2dc
  },
  btnDisabled: { opacity: 0.5, cursor: "not-allowed" },
  field: { display: "block", marginBottom: 12 },
  fieldLabel: { display: "block", fontSize: 13, color: "var(--muted)", marginBottom: 4 }, // #4b5563
  input: {
    width: "100%",
    boxSizing: "border-box",
    padding: "8px 10px",
    fontSize: 14,
    border: "1px solid var(--border)", // #cbd2dc
    borderRadius: "var(--radius-sm)", // 6 → 8px
    background: "var(--surface)", // explicit so dark inputs are legible
    color: "var(--fg)",
  },
  badge: {
    display: "inline-block",
    padding: "2px 8px",
    borderRadius: 999,
    fontSize: 12,
    fontWeight: 600,
  },
  progressTrack: {
    height: 10,
    background: "var(--surface-warm)", // #eef1f5
    borderRadius: 999,
    overflow: "hidden",
  },
  progressFill: { height: "100%", background: "var(--accent)" }, // #2563eb
  table: { width: "100%", borderCollapse: "collapse", fontSize: 14 },
  th: {
    textAlign: "left",
    borderBottom: "2px solid var(--border)", // #e2e5ea
    padding: "8px 10px",
    color: "var(--muted)", // #4b5563
  },
  errorPanel: {
    background: "color-mix(in oklab, var(--danger), transparent 92%)", // #fef2f2
    border: "1px solid color-mix(in oklab, var(--danger), transparent 55%)", // #fca5a5
    color: "var(--danger)", // #991b1b
    borderRadius: "var(--radius-sm)",
    padding: 12,
    fontSize: 13,
  },
  errorList: { margin: "8px 0 0", paddingLeft: 18 },
  requestId: { marginTop: 8, fontFamily: "monospace", fontSize: 11, opacity: 0.8 },
  empty: { color: "var(--muted)", fontStyle: "italic", padding: 16 }, // #6b7280
};

const badgeTone: Record<BadgeTone, CSSProperties> = {
  neutral: { background: "var(--surface-warm)", color: "var(--fg)" }, // #eef1f5 / #374151
  ok: { background: "color-mix(in oklab, var(--success), transparent 85%)", color: "var(--success)" }, // #dcfce7 / #166534
  warn: { background: "color-mix(in oklab, var(--warn), transparent 85%)", color: "var(--warn)" }, // #fef9c3 / #854d0e
  err: { background: "color-mix(in oklab, var(--danger), transparent 85%)", color: "var(--danger)" }, // #fee2e2 / #991b1b
  info: { background: "color-mix(in oklab, var(--accent), transparent 85%)", color: "var(--accent)" }, // #dbeafe / #1e40af
};
