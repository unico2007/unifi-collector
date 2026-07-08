// Global refresh coordination + a polling hook. Pages fetch on mount, on an
// interval, when their deps change, and whenever the header "Yenilə" button
// (or any caller of refresh()) fires a manual refresh. Background polls are
// silent — they don't reset the page to a loading state, they just swap in
// fresher data.

import { createContext, useCallback, useContext, useEffect, useRef, useState, ReactNode } from "react";

const DEFAULT_INTERVAL = 20000; // 20s

interface RefreshCtxValue {
  tick: number;
  refreshing: boolean;
  lastUpdated: Date | null;
  refresh: () => void;
  reportUpdate: () => void;
}

const Ctx = createContext<RefreshCtxValue>(null!);
export const useRefresh = () => useContext(Ctx);

export function RefreshProvider({ children }: { children: ReactNode }) {
  const [tick, setTick] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const refresh = useCallback(() => {
    setRefreshing(true);
    setTick((t) => t + 1);
    window.setTimeout(() => setRefreshing(false), 700);
  }, []);
  const reportUpdate = useCallback(() => setLastUpdated(new Date()), []);

  return <Ctx.Provider value={{ tick, refreshing, lastUpdated, refresh, reportUpdate }}>{children}</Ctx.Provider>;
}

/**
 * Fetch `fn` on mount, every `intervalMs`, when any `deps` change, and on every
 * global manual refresh. Returns the latest data (kept across background polls)
 * and a `loading` flag that is only true until the first successful fetch.
 */
export function usePolling<T>(fn: () => Promise<T>, deps: unknown[] = [], intervalMs = DEFAULT_INTERVAL) {
  const { tick, reportUpdate } = useRefresh();
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const d = await fnRef.current();
        if (!alive) return;
        setData(d);
        reportUpdate();
      } catch {
        // Keep the last good data on a failed poll rather than blanking the UI.
      } finally {
        if (alive) setLoading(false);
      }
    };
    load();
    const id = window.setInterval(load, intervalMs);
    return () => {
      alive = false;
      window.clearInterval(id);
    };
    // reportUpdate is stable (useCallback); spreading deps drives param-based refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, intervalMs, ...deps]);

  return { data, loading };
}
