# CareStack Voice Periodontal Charting

### **DSOLVE 2026** • DRISHTI • College of Engineering Trivandrum (CET)

**BUILD. SOLVE. DEMONSTRATE.**

|                   |                                           |
| ----------------- | ----------------------------------------- |
| **Problem:**      | Problem Statement 7  - Carestack Voice Charter  |
| **Team Name:**    | Innovatex                          |
| **Team Members:** | Amrutha T Ravi • Fuad Fysal • Shirin Shukkoor |
| **Institution:**  | College Of Engineering Trivandrum                   |
| **Live Demo:**    |  https://drive.google.com/file/d/1U3YW8sazTy5EC2E6Sn4jmaE90QSagJJt/view?usp=drivesdk               |
| **Pitch Video:**  | https://www.instagram.com/reel/DdcjpvShBnY/?stkn=MjZyOG9zNm9ubWVt           |

---

## Table of Contents

- [Problem Statement](#problem-statement)
- [Our Solution](#our-solution)
- [Key Features](#key-features)
- [Screenshots & Demo](#screenshots--demo)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
- [Usage / Demo Script](#usage--demo-script)
- [Limitations & Future Scope](#limitations--future-scope)
- [Team](#team)
- [Submission Checklist](#submission-checklist)

---

## Problem Statement

> ## Problem 26C: Voice Periodontal Charting
>
> Dental charting requires manual data entry which is time-consuming and interrupts clinical workflow. Cross-contamination risks increase when switching between patient care and typing. Existing voice solutions often suffer from high latency and poor accuracy with dental terminology.

### Why this matters

Manual periodontal charting takes valuable time away from patient care and requires either an assistant to record numbers or the hygienist to repeatedly break sterility to type. By automating this with a fast, accurate voice solution, solo practitioners can save time, increase production, and drastically reduce cross-contamination risks.

---

## Our Solution

We built a real-time, hands-free voice charting system for dental clinical documentation that integrates directly into CareStack's PMS. 

Our solution boasts a **sub-300ms** voice-to-UI latency by utilizing a strict **dual-track Hybrid APPS Router**. Instead of passing every command to a slow Large Language Model, 85% of standard clinical utterances are parsed locally via a deterministic Regex Fast-Path (`< 1ms`). Only complex, ambiguous, or conversational corrections (e.g., "no wait, that last one was a five") are routed to the Groq AI Slow-Path (`~80ms`) running Llama-3-8B. Combined with an optimistic React UI, the charting experience is flawless and feels completely instantaneous to the clinician.

---

## Key Features

- **Hybrid APPS Router** – Routes voice commands intelligently to achieve `<300ms` total latency. Regex Fast-Path (⚡) for standard commands and Groq AI Slow-Path (🧠) for natural language corrections.
- **Interactive Periodontal Chart** – Dual-arch SVG chart with color-coded pocket depths, bleeding dots, recession, furcation, and mobility tracking.
- **Optimistic UI with Zundo** – UI updates instantly with visual cues (amber rings) while async DB-Writer processes the backend transaction. Includes a 50-level undo history ("scratch that").
- **Live Voice Dashboard** – Real-time metrics including Charted/32, BOP%, Mean PD, active tooth highlights, and downloadable CSV reports.
- **HIPAA Compliant Architecture** – No audio storage, JWT scoped authentication, PostgreSQL append-only audit logs, and async Redis streams.

---

## Screenshots & Demo

| Screenshot                                            | Description                          |
| ----------------------------------------------------- | ------------------------------------ |
| [Screenshot 1](chart-ui.png)                          | Interactive Periodontal Chart & UI   |
| [Screenshot 2](dashboard.png)                         | Live Voice Session Dashboard & Stats |
| [Pitch Video]                                         | Link to your >30s social pitch video |

---

## Tech Stack

| Layer           | Technology                         | Why we chose it |
| --------------- | ---------------------------------- | --------------- |
| Frontend        | React 18, Vite, TypeScript         | Fast HMR, strong typing, and excellent component ecosystem for building complex SVG charts. |
| State Management| Zustand + zundo                    | Lightweight, optimistic UI updates and instant 50-level temporal undo capabilities. |
| Backend         | Go (Voice Session & DB Writer)     | High performance, efficient concurrency for WebSocket streaming, and sub-millisecond regex parsing. |
| Database        | PostgreSQL                         | Relational data integrity, HIPAA-compliant audit logging, and append-only rules. |
| Message Broker  | Redis                              | Low-latency Pub/Sub for live state updates and Redis Streams for async DB writes. |
| ML / AI         | Deepgram (STT), Groq (Llama-3-8B)  | Nova-2-Medical STT offers the best dental accuracy. Groq provides ultra-fast LLM inference (~80ms). |
| Infra / Hosting | Docker & Nginx                     | Easy containerized deployment and reliable WSS routing without buffering. |

---

## Getting Started

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (v24.0.0+)
- [Deepgram API key](https://console.deepgram.com) (Optional - for live mic)
- [Groq API key](https://console.groq.com) (Optional - for AI slow-path)

### Installation

Clone the repository and copy the example environment file:
```bash
git clone <repo>
cd "DSOLVE'26"
cp .env.example .env
```

Start the entire microservice stack using Docker Compose:
```bash
docker compose up --build -d
```
Wait ~30 seconds, then open `http://localhost` in your browser.

### Environment Variables

| Variable       | Description                       | Example                           |
| -------------- | --------------------------------- | --------------------------------- |
| `DEEPGRAM_API_KEY` | Deepgram STT Key (Leave empty for Mock Mode) | `your_deepgram_key`       |
| `GROQ_API_KEY`     | Groq AI Key (Leave empty for Regex-only mode)| `gsk_xxxxxxxxxxxxxxxxxxx` |
| `POSTGRES_PASSWORD`| Database password                 | `cs_dev_password`                 |
| `JWT_SECRET`       | Secret for scoped tokens          | `dev-jwt-secret`                  |

---

## Usage / Demo Script

1. **Boot** – Start the application via `docker compose up --build -d` and open `http://localhost`. If no API keys are provided, the system boots into offline **Mock Mode** looping through 50+ scripted realistic utterances.
2. **Start Session** – Click the microphone button to start a charting session. 
3. **Walkthrough step 1 (Basic Charting)** – Say *"tooth fourteen"* to navigate, then say *"three two four"* and *"bleeding"*. The UI immediately highlights tooth #14, plots the pocket depths, and adds bleeding markers using the Regex Fast-Path (⚡).
4. **Walkthrough step 2 (Corrections)** – Say *"make site two a five"* or *"scratch that"* to demonstrate the deterministic undo functionality.
5. **Highlight (The AI Slow-Path 🧠)** – Say a complex correction like *"no wait, that last one was wrong, it should be a six"*. The system will seamlessly route this to Groq Llama-3-8B and correct the UI intelligently within ~300ms.
6. **Wrap-up** – Show the Session Dashboard metrics updating in real-time, click "Download Report" to export a CSV, and emphasize how this allows a solo hygienist to completely ditch the keyboard.

---

## Limitations & Future Scope

### Known Limitations

- **Internet Dependency**: Live voice mode requires an active internet connection to stream audio to Deepgram and Groq.
- **AI Latency Variance**: While the Regex path is deterministic (`< 1ms`), the Groq AI path is subject to network jitter and can occasionally spike up to ~250ms.
- **Hardware**: Background noise in a busy dental clinic might require users to wear a directional headset microphone for optimal STT accuracy.

### Future Scope

- **Full Restorative Charting**: Expand the voice engine vocabulary to handle cavities, crowns, extractions, and multi-surface restorations (e.g., "MOD composite on tooth 4").
- **Multi-Lingual Support**: Leverage multi-lingual STT models to allow dental staff to chart in Spanish or other languages seamlessly.
- **Deep EHR Integration**: Direct automated syncing with external billing systems to automatically generate insurance codes (e.g., scaling and root planing) based on charted pocket depths.

---

## Team

| Name           | Role(s)                         | GitHub      | Email   |
| --------       | ------------------------------- | ---------   | ------- |
| Amrutha T Ravi | Full-stack / Backend / AI   | @Amruthatravi   |  amruthatravithanikkal@gmail.com |
| Fuad Fysal     | Frontend / UX Design        | @fuad-dotcom    | fuadfysaln@gmail.com |
| Shirin Shukkoor| Architecture / DevOps       | @ShirinS-techie |shirinshukur@gmail.com |
| Fuad Fysal     | Testing / Domain Logic      | @fuad-dotcom    |   fuadfysaln@gmail.com |

---

## Submission Checklist

**Before 6:00 AM (Code Freeze) – Sat, Sept 19th:**

- [x] Clean, runnable source code committed to this **public** repo
- [x] `README.md` fully filled in (all sections above)
- [ ] Pitch video (>30s, English) posted on team member's social profile tagging **@DrishtiCET** & **@CareStack** and link added above
- [x] All secrets/API keys removed from the repo
- [x] Quick-start verified from a fresh clone (`git clone` + run)
