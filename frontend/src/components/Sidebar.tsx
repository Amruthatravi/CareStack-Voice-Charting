import type { AuditRow, Chart, Meta } from "../types";
import { depthColor } from "./ChartArch";

const PATH_LABEL: Record<string, string> = {
  regex: "Regex fast-path",
  llm: "Groq AI slow-path",
  fallback: "Local fallback",
  init: "Ready",
};

const COMMANDS = [
  "tooth 14",
  "three two four",
  "bleeding / bop",
  "recession two",
  "furcation class two",
  "mobility one",
  "suppuration",
  "plaque positive",
  "next",
  "scratch that",
  "make site 2 a 5",
  "no wait, that was a five",
];

function summary(chart: Chart) {
  let sites = 0, bop = 0, plaque = 0, max = 0;
  for (const t of Object.values(chart.teeth)) {
    for (const s of [t.buccal, t.lingual]) {
      for (let i = 0; i < 3; i++) {
        if (s.pd[i] > 0) {
          sites++;
          if (s.bop[i]) bop++;
          if (s.plaque[i]) plaque++;
          max = Math.max(max, s.pd[i]);
        }
      }
    }
  }
  const pct = (n: number) => (sites ? `${Math.round((n / sites) * 100)}%` : "–");
  return { sites, bop: pct(bop), plaque: pct(plaque), max: max || "–" };
}

export function Sidebar({ meta, chart, audit }: { meta: Meta | null; chart: Chart; audit: AuditRow[] }) {
  const s = summary(chart);
  const path = meta?.path ?? "init";
  return (
    <aside className="side">
      <div className="card">
        <span className={`badge ${path}`}>
          {PATH_LABEL[path]}
          {meta && path !== "init" && <b>{meta.latencyMs < 10 ? meta.latencyMs.toFixed(1) : Math.round(meta.latencyMs)}ms</b>}
        </span>
        {meta && meta.messages.length > 0 && path !== "init" && <p className="said">{meta.messages.join(" · ")}</p>}
      </div>

      <div className="card">
        <h3>Chart summary</h3>
        <dl className="sum">
          <div><dt>Sites charted</dt><dd>{s.sites}</dd></div>
          <div><dt>Bleeding</dt><dd>{s.bop}</dd></div>
          <div><dt>Plaque</dt><dd>{s.plaque}</dd></div>
          <div><dt>Deepest pocket</dt><dd>{s.max}{typeof s.max === "number" ? " mm" : ""}</dd></div>
        </dl>
      </div>

      <div className="card">
        <h3>Pocket depth</h3>
        <ul className="legend">
          <li><i style={{ background: depthColor(2) }} />1–3mm · Healthy</li>
          <li><i style={{ background: depthColor(4) }} />4–5mm · Moderate</li>
          <li><i style={{ background: depthColor(6) }} />6mm · Deep</li>
          <li><i style={{ background: depthColor(7) }} />7+mm · Critical</li>
          <li><i className="ring" />Bleeding on probing</li>
          <li><i style={{ background: "#a855f7" }} />Suppuration</li>
          <li><i style={{ background: "#eab308" }} />Plaque</li>
        </ul>
      </div>

      <div className="card">
        <h3>Voice commands</h3>
        <ul className="cmds">
          {COMMANDS.map((c) => (
            <li key={c}>"{c}"</li>
          ))}
        </ul>
      </div>

      <div className="card">
        <h3>Saved audit log</h3>
        {audit.length === 0 ? (
          <p className="muted">Nothing saved yet. Entries appear here once written to PostgreSQL.</p>
        ) : (
          <ol className="audit">
            {audit.map((a) => (
              <li key={a.id}>
                <span className="seq">#{a.seq}</span>
                <span className="raw">{a.raw || "(empty)"}</span>
                <span className={`tag ${a.kind}`}>{a.kind}</span>
              </li>
            ))}
          </ol>
        )}
      </div>
    </aside>
  );
}
