// Helix OTA — minimal server-state query hook (design §3.1, §7.1).
// A thin stand-in for the "query-cache layer" the design calls for (request lifecycle,
// loading/error state, manual + interval refetch). Kept self-contained so the scaffold has
// no unverified external data-fetching dependency; the real brick/library swaps in here.

import { useCallback, useEffect, useRef, useState } from "react";

export interface QueryResult<T> {
  data: T | null;
  error: unknown;
  loading: boolean;
  refetch(): void;
}

export function useApi<T>(
  fetcher: () => Promise<T>,
  opts: { enabled?: boolean; intervalMs?: number; deps?: unknown[] } = {},
): QueryResult<T> {
  const { enabled = true, intervalMs } = opts;
  const deps = opts.deps ?? [];

  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [loading, setLoading] = useState<boolean>(enabled);

  // Keep the latest fetcher without retriggering the effect every render.
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const run = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await fetcherRef.current();
      setData(result);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      return;
    }
    void run();
    if (intervalMs && intervalMs > 0) {
      const id = setInterval(() => void run(), intervalMs);
      return () => clearInterval(id);
    }
    return undefined;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, intervalMs, run, ...deps]);

  return { data, error, loading, refetch: run };
}
