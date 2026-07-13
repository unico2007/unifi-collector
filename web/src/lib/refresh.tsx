// Global refresh coordination + a polling hook. Pages fetch on mount, on an
// interval, when their deps change, and whenever the header "Yenilə" button
// (or any caller of refresh()) fires a manual refresh. Background polls are
// silent — they don't reset the page to a loading state, they just swap in
// fresher data. Polls report success/failure so the header can show a live
// connection status, and pause while the tab is hidden.

import { createContext, useCallback, useContext, useEffect, useRef, useState, ReactNode } from "react";

const DEFAULT_INTERVAL = 20000; // 20s
const STALE_AFTER = 45000; // no successful poll for this long => "stale"

export type LiveStatus = "live" | "error" | "stale";

interface RefreshCtxValue {
  tick: number;
  refreshing: boolean;
  lastUpdated: Date | null;
  status: LiveStatus;
  refresh: () => void;
  reportUpdate: () => void;
  reportError: () => void;
}

const Ctx = createContext<RefreshCtxValue>(null!);
export const useRefresh = () => useContext(Ctx);

export function RefreshProvider({ children }: { children: ReactNode }) {
  const [tick, setTick] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [status, setStatus] = useState<LiveStatus>("live");
  const lastOkRef = useRef<number>(Date.now());

  const refresh = useCallback(() => {
    setRefreshing(true);
    setTick((t) => t + 1);
    window.setTimeout(() => setRefreshing(false), 700);
  }, []);

  const reportUpdate = useCallback(() => {
    lastOkRef.current = Date.now();
    setLastUpdated(new Date());
    setStatus("live");
  }, []);

  // A failed poll only degrades the dot when data is actually going stale —
  // a single blip while another widget still updates fine stays green.
  const reportError = useCallback(() => {
    if (Date.now() - lastOkRef.current > STALE_AFTER) setStatus("error");
  }, []);

  // Watchdog: no successful update for a while (e.g. all polls failing or
  // paused) => show "stale" instead of a frozen green dot.
  useEffect(() => {
    const id = window.setInterval(() => {
      if (Date.now() - lastOkRef.current > STALE_AFTER) {
        setStatus((s) => (s === "error" ? s : "stale"));
      }
    }, 10000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <Ctx.Provider value={{ tick, refreshing, lastUpdated, status, refresh, reportUpdate, reportError }}>
      {children}
    </Ctx.Provider>
  );
}

/**
 * Fetch `fn` on mount, every `intervalMs`, when any `deps` change, and on every
 * global manual refresh. Returns the latest data (kept across background polls)
 * and a `loading` flag that is only true until the first successful fetch.
 * While the tab is hidden the interval skips; on return to the tab it fetches
 * immediately, so 10 idle tabs don't hammer the BFF.
 */
export function usePolling<T>(fn: () => Promise<T>, deps: unknown[] = [], intervalMs = DEFAULT_INTERVAL) {
  const { tick, reportUpdate, reportError } = useRefresh();
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
        if (alive) reportError();
      } finally {
        if (alive) setLoading(false);
      }
    };
    load();
    const id = window.setInterval(() => {
      if (!document.hidden) load();
    }, intervalMs);
    const onVisible = () => {
      if (!document.hidden) load();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      alive = false;
      window.clearInterval(id);
      document.removeEventListener("visibilitychange", onVisible);
    };
    // reportUpdate/reportError are stable (useCallback); deps drive param-based refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, intervalMs, ...deps]);

  return { data, loading };
}
