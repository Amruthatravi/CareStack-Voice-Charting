export type Surface = "buccal" | "lingual";

export interface Side {
  pd: [number, number, number];
  rec: [number, number, number];
  bop: [boolean, boolean, boolean];
  supp: [boolean, boolean, boolean];
  plaque: [boolean, boolean, boolean];
}

export interface Tooth {
  buccal: Side;
  lingual: Side;
  mobility: number;
  furcation: number;
  missing: boolean;
}

export interface Cursor {
  tooth: number;
  surface: Surface;
  site: number;
  end: boolean;
}

export interface Chart {
  teeth: Record<string, Tooth>;
  cursor: Cursor;
  last?: { tooth: number; surface: Surface; site: number };
}

export type Path = "init" | "regex" | "llm" | "fallback";

export interface Meta {
  path: Path;
  latencyMs: number;
  messages: string[];
  heard: string;
  seq: number;
}

export interface StateMsg {
  sessionId: string;
  chart: Chart;
  meta: Meta;
}

export interface SessionInfo {
  id: string;
  patientLabel: string;
  createdAt: string;
}

export interface AuditRow {
  id: number;
  seq: number;
  kind: string;
  raw: string;
  source: string;
  latencyMs: number;
  occurredAt: string;
}
