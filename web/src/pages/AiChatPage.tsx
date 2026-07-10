import { useEffect, useRef, useState } from "react";
import { api, AiChat } from "../lib/api";

interface Msg {
  role: "user" | "ai";
  text: string;
  meta?: { source: string; query: string; result?: string };
  error?: boolean;
}

const suggestions = [
  "Neçə cihaz var?",
  "Hansı cihazlar offline-dır?",
  "AP offline olsa nə yoxlamalıyam?",
  "CPU-su ən yüksək cihaz hansıdır?",
];

const sourceLabel: Record<string, string> = {
  prometheus: "Prometheus (metrik)",
  loki: "Loki (log)",
  knowledge: "Bilik bazası (RAG)",
  troubleshoot: "Problem-həll (RAG + log)",
};

export default function AiChatPage() {
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [summary, setSummary] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.aiSummary().then(setSummary);
  }, []);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [msgs, busy]);

  async function ask(question: string) {
    const q = question.trim();
    if (!q || busy) return;
    setInput("");
    setMsgs((m) => [...m, { role: "user", text: q }]);
    setBusy(true);
    try {
      const r: AiChat = await api.aiChat(q);
      setMsgs((m) => [
        ...m,
        { role: "ai", text: r.answer, meta: { source: r.source, query: r.query, result: r.result } },
      ]);
    } catch {
      setMsgs((m) => [...m, { role: "ai", text: "Cavab alına bilmədi. AI servisini yoxlayın.", error: true }]);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col h-full max-w-3xl mx-auto">
      {/* Daily summary banner */}
      {summary && (
        <div className="card p-4 mb-4 bg-brand-50/40 border-brand-100">
          <div className="flex items-center gap-2 text-sm font-medium text-brand-700 mb-1">
            <Spark /> AI xülasə
          </div>
          <div className="text-sm text-ink/80">{summary}</div>
        </div>
      )}

      {/* Conversation */}
      <div className="flex-1 overflow-y-auto space-y-4 pr-1">
        {msgs.length === 0 && (
          <div className="text-center mt-10 space-y-4">
            <div className="w-12 h-12 mx-auto rounded-2xl bg-brand-500 text-white grid place-items-center">
              <Spark className="w-6 h-6" />
            </div>
            <div>
              <div className="font-semibold">Unico AI köməkçisi</div>
              <div className="text-sm text-muted mt-1">Şəbəkə haqqında Azərbaycanca sual verin — cavabı canlı metrik və loglardan alınır.</div>
            </div>
            <div className="flex flex-wrap gap-2 justify-center">
              {suggestions.map((s) => (
                <button key={s} onClick={() => ask(s)} className="btn text-xs">{s}</button>
              ))}
            </div>
          </div>
        )}

        {msgs.map((m, i) => (
          <div key={i} className={`flex ${m.role === "user" ? "justify-end" : "justify-start"}`}>
            <div className={m.role === "user" ? "max-w-[80%]" : "max-w-[85%] w-full"}>
              <div
                className={`px-4 py-2.5 rounded-2xl text-sm whitespace-pre-wrap ${
                  m.role === "user"
                    ? "bg-brand-500 text-white rounded-br-sm"
                    : m.error
                    ? "bg-red-50 text-red-700 rounded-bl-sm"
                    : "card rounded-bl-sm"
                }`}
              >
                {m.text}
              </div>
              {m.meta?.query && <QueryDetail meta={m.meta} />}
            </div>
          </div>
        ))}

        {busy && (
          <div className="flex justify-start">
            <div className="card px-4 py-3 rounded-2xl rounded-bl-sm">
              <Dots />
            </div>
          </div>
        )}
        <div ref={endRef} />
      </div>

      {/* Composer */}
      <form
        onSubmit={(e) => {
          e.preventDefault();
          ask(input);
        }}
        className="mt-4 flex items-center gap-2"
      >
        <input
          className="input flex-1"
          placeholder="Sual yazın... (məs: Neçə klient qoşuludur?)"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          disabled={busy}
        />
        <button type="submit" className="btn btn-primary" disabled={busy || !input.trim()}>
          Göndər
        </button>
      </form>
    </div>
  );
}

function QueryDetail({ meta }: { meta: { source: string; query: string; result?: string } }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="mt-1.5 ml-1">
      <button onClick={() => setOpen((o) => !o)} className="text-[11px] text-muted hover:text-brand-600 inline-flex items-center gap-1">
        <span>{open ? "▾" : "▸"}</span>
        <span className="pill bg-slate-100 text-slate-500">{sourceLabel[meta.source] ?? meta.source ?? "sorğu"}</span>
        <span>sorğunu göstər</span>
      </button>
      {open && (
        <div className="mt-1 space-y-1">
          <pre className="text-[11px] font-mono bg-slate-900 text-slate-100 rounded-lg px-3 py-2 overflow-x-auto">{meta.query}</pre>
          {meta.result && (
            <pre className="text-[11px] font-mono bg-slate-50 text-slate-600 border border-line rounded-lg px-3 py-2 overflow-x-auto whitespace-pre-wrap max-h-40">{meta.result}</pre>
          )}
        </div>
      )}
    </div>
  );
}

function Spark({ className = "w-4 h-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className={className}>
      <path d="M12 3l1.9 4.8L18 9l-4.1 1.2L12 15l-1.9-4.8L6 9l4.1-1.2zM19 14l.9 2.1L22 17l-2.1.9L19 20l-.9-2.1L16 17l2.1-.9z" />
    </svg>
  );
}

function Dots() {
  return (
    <div className="flex gap-1">
      {[0, 1, 2].map((i) => (
        <span key={i} className="w-1.5 h-1.5 rounded-full bg-muted animate-bounce" style={{ animationDelay: `${i * 0.15}s` }} />
      ))}
    </div>
  );
}
