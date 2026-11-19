import type { Chart, Side } from "../types";

export const depthColor = (pd: number) =>
  pd >= 7 ? "#7f1d1d" : pd === 6 ? "#ef4444" : pd >= 4 ? "#f59e0b" : "#16a34a";

const W = 56; // column width
const H = 64; // graph height
const XS = [9, 28, 47];
const yFor = (pd: number) => Math.min(H - 6, 12 + pd * 4.6);

function Graph({ side }: { side: Side }) {
  const pts = side.pd
    .map((pd, i) => (pd > 0 ? { x: XS[i], y: yFor(pd), pd, i } : null))
    .filter((p): p is { x: number; y: number; pd: number; i: number } => p !== null);
  const worst = Math.max(0, ...side.pd);
  return (
    <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} role="img" aria-label={`Pocket depths ${side.pd.join(", ")}`}>
      {pts.length > 1 && (
        <polyline
          points={pts.map((p) => `${p.x},${p.y}`).join(" ")}
          fill="none"
          stroke={depthColor(worst)}
          strokeWidth="2"
          strokeLinejoin="round"
        />
      )}
      {side.bop.map((b, i) => b && side.pd[i] === 0 && <circle key={`b${i}`} cx={XS[i]} cy={H / 2} r="5" fill="#e11d48" />)}
      {pts.map((p) => (
        <g key={p.i}>
          {side.bop[p.i] && <circle cx={p.x} cy={p.y} r="6" fill="none" stroke="#e11d48" strokeWidth="2" />}
          <circle cx={p.x} cy={p.y} r="3" fill={depthColor(p.pd)} />
          <text x={p.x} y={p.y - 6} textAnchor="middle" fontSize="11" fontWeight="600" fill={depthColor(p.pd)}>
            {p.pd}
          </text>
        </g>
      ))}
    </svg>
  );
}

function Flags({ side }: { side: Side }) {
  return (
    <div className="flags">
      {[0, 1, 2].map((i) => (
        <div className={`flag-site${side.bop[i] ? " has-bop" : ""}`} key={i}>
          <span className={`dot ${side.bop[i] ? "bop" : "off"}`} title={side.bop[i] ? "Bleeding on probing" : ""} />
          <span className={`dot ${side.supp[i] ? "supp" : "off"}`} title={side.supp[i] ? "Suppuration" : ""} />
          <span className={`dot ${side.plaque[i] ? "plaque" : "off"}`} title={side.plaque[i] ? "Plaque" : ""} />
        </div>
      ))}
    </div>
  );
}

const Rec = ({ side }: { side: Side }) => (
  <div className="rec">
    {side.rec.map((r, i) => (
      <span key={i}>{r > 0 ? r : ""}</span>
    ))}
  </div>
);

const EMPTY: Side = {
  pd: [0, 0, 0],
  rec: [0, 0, 0],
  bop: [false, false, false],
  supp: [false, false, false],
  plaque: [false, false, false],
};

export function ChartArch({ title, teeth, chart, onPick }: { title: string; teeth: number[]; chart: Chart; onPick: (n: number) => void }) {
  return (
    <section className="arch">
      <h2>{title}</h2>
      <div className="arch-scroll">
        <div className="arch-grid" style={{ gridTemplateColumns: `repeat(${teeth.length}, ${W}px)` }}>
          {teeth.map((n) => {
            const t = chart.teeth[String(n)];
            const b = t?.buccal ?? EMPTY;
            const l = t?.lingual ?? EMPTY;
            const active = !chart.cursor.end && chart.cursor.tooth === n;
            const aSurf = chart.cursor.surface;
            return (
              <div key={n} className={`col${t?.missing ? " missing" : ""}${active ? " active" : ""}`}>
                <div className={`surf${active && aSurf === "buccal" ? " cur" : ""}`}>
                  <Graph side={b} />
                  <Flags side={b} />
                  <Rec side={b} />
                </div>
                <button className="num" onClick={() => onPick(n)} aria-label={`Tooth ${n}`} aria-current={active}>
                  {n}
                  {t && (t.mobility > 0 || t.furcation > 0) && (
                    <small>
                      {t.mobility > 0 ? `M${t.mobility}` : ""} {t.furcation > 0 ? `F${t.furcation}` : ""}
                    </small>
                  )}
                </button>
                <div className={`surf${active && aSurf === "lingual" ? " cur" : ""}`}>
                  <Rec side={l} />
                  <Flags side={l} />
                  <Graph side={l} />
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
