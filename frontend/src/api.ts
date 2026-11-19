import type { AuditRow, SessionInfo, StateMsg } from "./types";

async function j<T>(res: Response): Promise<T> {
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
  return res.json() as Promise<T>;
}

export const createSession = (patientLabel: string) =>
  fetch("/api/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ patientLabel }),
  }).then(j<SessionInfo>);

export const getSession = (id: string) => fetch(`/api/sessions/${id}`).then(j<StateMsg>);

export const postUtterance = (id: string, text: string) =>
  fetch(`/api/sessions/${id}/utterances`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  }).then(j<StateMsg>);

export const getAudit = (id: string, limit = 12) =>
  fetch(`/api/sessions/${id}/audit?limit=${limit}`).then(j<AuditRow[]>);

export const exportUrl = (id: string) => `/api/sessions/${id}/export`;

export type ConnState = "connecting" | "open" | "closed";

/** WebSocket with automatic reconnect. */
export function connect(
  sessionId: string,
  onState: (s: StateMsg) => void,
  onConn: (c: ConnState) => void,
) {
  let ws: WebSocket | null = null;
  let stopped = false;
  let retry = 0;

  const open = () => {
    onConn("connecting");
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(`${proto}://${location.host}/ws?session=${sessionId}`);
    ws.onopen = () => {
      retry = 0;
      onConn("open");
    };
    ws.onmessage = (e) => {
      const m = JSON.parse(e.data);
      if (m.type === "state") onState(m.data as StateMsg);
    };
    ws.onclose = () => {
      onConn("closed");
      if (!stopped) setTimeout(open, Math.min(5000, 500 * 2 ** retry++));
    };
  };
  open();

  return {
    send(text: string): boolean {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "utterance", text }));
        return true;
      }
      return false;
    },
    close() {
      stopped = true;
      ws?.close();
    },
  };
}
