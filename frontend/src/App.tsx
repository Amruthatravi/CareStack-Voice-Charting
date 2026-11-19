import { useCallback, useEffect, useRef, useState } from "react";
import { ChartArch } from "./components/ChartArch";
import { Sidebar } from "./components/Sidebar";
import { connect, createSession, exportUrl, getAudit, postUtterance, type ConnState } from "./api";
import { VoiceEngine } from "./voice";
import type { AuditRow, Chart, Meta, StateMsg } from "./types";

const UPPER = Array.from({ length: 16 }, (_, i) => i + 1);
const LOWER = Array.from({ length: 16 }, (_, i) => 32 - i);
const EMPTY: Chart = { teeth: {}, cursor: { tooth: 1, surface: "buccal", site: 0, end: false } };

type Mic = "idle" | "loading" | "listening" | "error";

export default function App() {
  const [sid, setSid] = useState<string | null>(localStorage.getItem("perio.session"));
  const [label, setLabel] = useState(localStorage.getItem("perio.label") ?? "");
  const [chart, setChart] = useState<Chart>(EMPTY);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [conn, setConn] = useState<ConnState>("connecting");
  const [mic, setMic] = useState<Mic>("idle");
  const [micErr, setMicErr] = useState("");
  const [partial, setPartial] = useState("");
  const [heard, setHeard] = useState<string[]>([]);
  const [audit, setAudit] = useState<AuditRow[]>([]);
  const [typed, setTyped] = useState("");

  const wsRef = useRef<ReturnType<typeof connect> | null>(null);
  const voiceRef = useRef(new VoiceEngine());
  const sidRef = useRef(sid);
  sidRef.current = sid;
  const auditTimer = useRef<number | undefined>(undefined);

  const refreshAudit = useCallback(() => {
    window.clearTimeout(auditTimer.current);
    auditTimer.current = window.setTimeout(() => {
      if (sidRef.current) getAudit(sidRef.current).then(setAudit).catch(() => {});
    }, 700);
  }, []);

  const onState = useCallback(
    (s: StateMsg) => {
      setChart(s.chart);
      setMeta(s.meta);
      if (s.meta.path !== "init") {
        refreshAudit();
        if (s.meta.heard) setHeard((h) => [s.meta.heard, ...h].slice(0, 4));
      }
    },
    [refreshAudit],
  );

  const newSession = useCallback(async (patient: string) => {
    const s = await createSession(patient);
    localStorage.setItem("perio.session", s.id);
    localStorage.setItem("perio.label", patient);
    setLabel(patient);
    setChart(EMPTY);
    setMeta(null);
    setAudit([]);
    setHeard([]);
    setSid(s.id);
  }, []);

  // Create a session on first load.
  useEffect(() => {
    if (!sid) void newSession("Walk-in patient");
  }, [sid, newSession]);

  // Open the live socket whenever the session changes.
  useEffect(() => {
    if (!sid) return;
    const ws = connect(sid, onState, setConn);
    wsRef.current = ws;
    getAudit(sid).then(setAudit).catch(() => {
      // Stored session no longer exists (e.g. database was reset): start a fresh one.
      localStorage.removeItem("perio.session");
      setSid(null);
    });
    return () => ws.close();
  }, [sid, onState]);

  const send = useCallback(
    (text: string) => {
      const id = sidRef.current;
      if (!id || !text.trim()) return;
      if (!wsRef.current?.send(text)) postUtterance(id, text).then(onState).catch(() => {});
    },
    [onState],
  );

  const toggleMic = useCallback(async () => {
    if (mic === "listening" || mic === "loading") {
      voiceRef.current.stop();
      setMic("idle");
      setPartial("");
      return;
    }
    setMicErr("");
    try {
      await voiceRef.current.start({ onPartial: setPartial, onFinal: send }, () => setMic("loading"));
      setMic("listening");
    } catch (e) {
      setMic("error");
      setMicErr(
        e instanceof Error && /model|404|fetch|tar/i.test(e.message)
          ? "Speech model not found. Run `npm run setup:model` in frontend/ (Docker does this automatically)."
          : "Microphone unavailable. Allow mic access (needs localhost or https), or type commands below.",
      );
    }
  }, [mic, send]);

  // Space toggles the mic unless typing.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (e.code === "Space" && tag !== "INPUT" && tag !== "BUTTON") {
        e.preventDefault();
        void toggleMic();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggleMic]);

  useEffect(() => () => voiceRef.current.stop(), []);

  const pick = (n: number) => send(`tooth ${n}`);
  const micText = { idle: "Start", loading: "Loading speech model…", listening: "Listening · stop", error: "Retry" }[mic];

  return (
    <div className="app">
      <header className="top">
        <div>
          <h1>Perio Assist</h1>
          <p className="muted">
            {label || "Patient"} · chart {sid ? sid.slice(0, 8) : "…"} ·{" "}
            <span className={`conn ${conn}`}>{conn === "open" ? "live" : conn === "connecting" ? "connecting" : "reconnecting"}</span>
          </p>
        </div>
        <div className="actions">
          <button
            className="ghost"
            onClick={() => {
              const p = window.prompt("Patient name or ID for the new chart:", "");
              if (p !== null) void newSession(p.trim() || "Walk-in patient");
            }}
          >
            New chart
          </button>
          {sid && (
            <a className="ghost" href={exportUrl(sid)} download>
              Export JSON
            </a>
          )}
          <button className={`mic ${mic}`} onClick={() => void toggleMic()} aria-pressed={mic === "listening"}>
            <span className="pulse" />
            {micText}
          </button>
        </div>
      </header>

      <div className="layout">
        <main>
          <ChartArch title="Upper Arch (1-16)" teeth={UPPER} chart={chart} onPick={pick} />
          <ChartArch title="Lower Arch (32-17)" teeth={LOWER} chart={chart} onPick={pick} />

          <div className="transcript" aria-live="polite">
            <p className="partial">{partial || (mic === "listening" ? "Listening…" : "Press Space or Start, then say “tooth 14, three two four, bleeding”.")}</p>
            {heard.length > 0 && <p className="heard">{heard.join("  ·  ")}</p>}
            {micErr && <p className="err">{micErr}</p>}
            {chart.cursor.end && <p className="err">End of arch reached. Say “tooth N” or switch surface.</p>}
          </div>

          <div className="status-line">
            Cursor: tooth <b>{chart.cursor.tooth}</b> · {chart.cursor.surface} · site <b>{chart.cursor.site + 1}</b>
          </div>

          <div className="typed">
            <input
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  send(typed);
                  setTyped("");
                }
              }}
              placeholder="Type a command if the room is too loud (Enter to send)"
              aria-label="Type a command"
            />
          </div>
        </main>
        <Sidebar meta={meta} chart={chart} audit={audit} />
      </div>
    </div>
  );
}
