// Helix OTA — LoginScreen (design §9.1).
// OAuth2 ROPC: POST /auth/login -> tokens established by AuthContext.

import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";
import { Button, Card, ErrorPanel, Field, TextInput } from "../components/ui";

interface LocationState {
  from?: string;
}

export function LoginScreen() {
  const { login, sessionExpiredNotice } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as LocationState | null)?.from ?? "/";

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);

  // Client validation is UX only — server remains authoritative (design §9.1).
  const canSubmit = username.trim().length > 0 && password.length > 0 && !submitting;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    try {
      await login(username, password);
      navigate(from, { replace: true });
    } catch (err) {
      // 401 -> "invalid credentials" without revealing which field; 429 -> retry-after.
      if (err instanceof ApiError && err.status === 401) {
        setError(new ApiError(401, "UNAUTHENTICATED", "Invalid username or password."));
      } else {
        setError(err);
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: "8vh auto" }}>
      <Card title="Helix OTA — operator login">
        {sessionExpiredNotice ? (
          <div style={{ marginBottom: 12, color: "#854d0e" }}>
            Your session expired. Please sign in again.
          </div>
        ) : null}
        <form onSubmit={onSubmit}>
          <Field label="Username (email)">
            <TextInput
              value={username}
              onChange={setUsername}
              autoComplete="username"
              placeholder="operator@example.com"
            />
          </Field>
          <Field label="Password">
            <TextInput
              value={password}
              onChange={setPassword}
              type="password"
              autoComplete="current-password"
            />
          </Field>
          {error ? <ErrorPanel error={error} /> : null}
          <div style={{ marginTop: 12 }}>
            <Button type="submit" disabled={!canSubmit}>
              {submitting ? "Signing in…" : "Sign in"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}
