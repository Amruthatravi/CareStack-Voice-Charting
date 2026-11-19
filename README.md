# Perio Assist

Real-time, hands-free periodontal charting by voice. Built from the PerioVoice idea
(https://github.com/joellijo32/Perio-Assist), which is a browser-only Svelte app with no
backend. This project adds a real backend: **Go + Redis Streams + PostgreSQL**, with a
**React/TypeScript** frontend and **on-device Vosk** speech recognition.

```
 Browser (React)                         Go backend                          Storage
┌──────────────────────┐   text only   ┌─────────────────────────────┐
│ mic -> Vosk (WASM)   │ ───────────►  │ 1. regex fast path (~0.3ms) │
│ audio never leaves   │   WebSocket   │ 2. ambiguous? -> Groq LLM   │──► Redis: live chart state
│ the device           │ ◄───────────  │    (rate-limited, cached,   │
│ live chart UI        │  chart patch  │     validated; fallback)    │──► Redis Stream (XADD)
└──────────────────────┘               │ 3. chart state machine      │          │ consumer group
                                       └─────────────────────────────┘          ▼
                                                                      PostgreSQL append-only audit_log
                                                                      + chart_snapshots
```

## Run it

```bash
cp .env.example .env        # optional: add GROQ_API_KEY
docker compose up --build   # first build downloads the 41 MB speech model
```
Open http://localhost:3000, click **Start** (or press Space), allow the microphone, and say:
`tooth fourteen`, `three two four`, `bleeding`, `no wait that was a five`, `scratch that`.
A text box under the chart accepts the same commands if the room is noisy.

Microphone access needs `localhost` or HTTPS.

### Without Docker (development)
```bash
# terminal 1: Redis + Postgres (any local instances), then
cd backend && DATABASE_URL=postgres://perio:perio@localhost:5432/perio?sslmode=disable go run ./cmd/server
# terminal 2
cd frontend && npm install && npm run setup:model && npm run dev     # http://localhost:5173
```

## How it works

**Fast path (regex/keyword parser, `backend/internal/parser`)** handles routine speech in microseconds:
`tooth 14`, `three two four`, `bleeding`/`bop`, `suppuration`, `plaque positive`, `recession two`,
`furcation class two`, `mobility one`, `next`, `buccal`/`lingual`, `missing`, `clear tooth`,
`scratch that`, `make site 2 a 5`, `change last to 5`. Several commands can be in one utterance.

**Slow path (Groq, Llama-3-8B-class model)** is used only for ambiguous spoken corrections such as
"no wait, that was a five". The model returns one strictly validated JSON action; it can never emit
anything outside the whitelist (`set_pd`, `set_flag`, `set_recession`, `goto`, `undo`, `none`) or out-of-range values.

**Staying under Groq's rate limit** (`backend/internal/llm`):
only ambiguous utterances are sent; global token bucket (`LLM_RPM`, default 20/min) plus a per-session
minimum gap; answers cached 10 min by (text, cursor context); an HTTP 429 opens a circuit for `Retry-After`;
2.5 s timeout, no retries. When the LLM is off, rate-limited or slow, a deterministic heuristic applies the correction
(`path = fallback`), so charting never stalls and works with no API key at all.

**Chart state machine (`backend/internal/chart`)**: cursor walks 1→16 then 32→17, three sites per tooth, skips missing teeth.
Every mutation is recorded as an old/new `Change`, so `scratch that` is an exact undo, including the cursor.
Flags (`bleeding`, `plaque`, `suppuration`, `recession`) apply to the **last site written** unless you say `site 2` or `on lingual`.

**Redis Streams → PostgreSQL**: the request path only updates live state in Redis and does one `XADD`.
A consumer-group worker batches events into Postgres in one transaction, then `XACK`s. Delivery is at-least-once,
and `event_id UNIQUE` makes writes idempotent. Failed batches stay pending and are re-claimed (`XAUTOCLAIM`);
undecodable messages go to a dead-letter stream. `audit_log` is **append-only**, enforced by database triggers
(UPDATE / DELETE / TRUNCATE raise an error). `chart_snapshots` lets a session recover if Redis is flushed.

**Privacy**: Vosk runs in the browser. Only recognised *text* reaches the backend (and only ambiguous text reaches Groq).

## API

| Method | Path | |
|---|---|---|
| POST | `/api/sessions` `{patientLabel}` | create a chart session |
| GET | `/api/sessions` | recent sessions |
| GET | `/api/sessions/{id}` | current chart |
| POST | `/api/sessions/{id}/utterances` `{text}` | process an utterance (same as the WebSocket) |
| GET | `/api/sessions/{id}/audit?limit=` | durable audit log from Postgres |
| GET | `/api/sessions/{id}/export` | chart as downloadable JSON |
| WS | `/ws?session={id}` | send `{type:"utterance",text}`, receive `{type:"state",data}` |
| GET | `/healthz` | Postgres / Redis / LLM status |

Try it with no microphone:
```bash
SID=$(curl -s -XPOST localhost:8080/api/sessions -d '{}' | jq -r .id)
curl -s -XPOST localhost:8080/api/sessions/$SID/utterances -d '{"text":"tooth 14 three two three bleeding"}' | jq .meta
```

## Tests
```bash
cd backend && go test ./...      # parser, chart state machine/undo, Groq client (mock server: cache, 429 circuit, validation)
cd frontend && npm run build     # type-check + production build
```

## Layout
```
backend/   cmd/server  internal/{parser,chart,llm,service,stream,store,api,event,config}
frontend/  src/{App.tsx,voice.ts,api.ts,components/}  scripts/get-model.sh  nginx.conf
docker-compose.yml  .env.example
```

## Known limits
- Charting order is one arch pass per surface (1→16, 32→17); say `buccal`/`lingual` to switch surface.
- English vocabulary only (~80 words, grammar-constrained). Pocket depths are spoken as single digits (`10+` via `make site 2 a 10`).
- Session state is serialised per session inside one backend process; to run several backend replicas, add sticky routing or a Redis lock.
- `AUDIT` data is patient-linked health data: add authentication and TLS before any real clinical use.
