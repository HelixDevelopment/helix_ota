// Helix OTA dashboard — unit tests for the useApi server-state hook.
// Asserts the loading -> data and loading -> error transitions, the enabled=false
// short-circuit, and manual refetch — the exact state machine every screen relies on.

import { describe, it, expect, vi } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useApi } from "./useApi";

describe("useApi", () => {
  it("starts loading, then resolves to data", async () => {
    const fetcher = vi.fn(() => Promise.resolve({ items: [1, 2, 3] }));
    const { result } = renderHook(() => useApi(fetcher));

    // Initial synchronous render: enabled -> loading true, no data/error yet.
    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual({ items: [1, 2, 3] });
    expect(result.current.error).toBeNull();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("captures a rejected fetcher into error and clears loading", async () => {
    const boom = new Error("boom");
    const fetcher = vi.fn(() => Promise.reject(boom));
    const { result } = renderHook(() => useApi(fetcher));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe(boom);
    expect(result.current.data).toBeNull();
  });

  it("does not fetch and is not loading when disabled", async () => {
    const fetcher = vi.fn(() => Promise.resolve("x"));
    const { result } = renderHook(() => useApi(fetcher, { enabled: false }));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(fetcher).not.toHaveBeenCalled();
    expect(result.current.data).toBeNull();
  });

  it("re-runs the fetcher on manual refetch", async () => {
    let n = 0;
    const fetcher = vi.fn(() => Promise.resolve(++n));
    const { result } = renderHook(() => useApi(fetcher));

    await waitFor(() => expect(result.current.data).toBe(1));
    await act(async () => {
      result.current.refetch();
    });
    await waitFor(() => expect(result.current.data).toBe(2));
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
