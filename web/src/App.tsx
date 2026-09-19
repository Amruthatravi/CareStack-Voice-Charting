import React from 'react'
import { useChartStore } from './store/chartStore'
import { useVoiceSession } from './hooks/useVoiceSession'
import { useChartWebSocket } from './hooks/useChartWebSocket'
import { PeriodontalChart } from './components/PeriodontalChart'
import { VoiceControl } from './components/VoiceControl'
import { LiveTranscript } from './components/VoiceControl/LiveTranscript'
import { SessionDashboard } from './components/SessionDashboard'

export default function App() {
  const teeth = useChartStore(s => s.teeth)
  const session = useChartStore(s => s.session)
  const transcriptHistory = useChartStore(s => s.transcriptHistory)

  const { startSession, stopSession, analyserRef } = useVoiceSession()
  const { handleMessage } = useChartWebSocket()

  const handleStart = () => startSession(handleMessage)
  const handleStop = () => stopSession()

  return (
    <div className="min-h-screen bg-slate-100 flex flex-col">
      {/* Header */}
      <header className="bg-sky-700 text-white px-6 py-3 flex items-center justify-between shadow-md">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-white rounded-full flex items-center justify-center">
            <span className="text-sky-700 font-bold text-sm">CS</span>
          </div>
          <div>
            <h1 className="font-semibold text-lg leading-tight">CareStack Voice Charting</h1>
            <p className="text-sky-200 text-xs">Real-Time Periodontal Documentation · Sub-300ms</p>
          </div>
        </div>
        <div className="text-sm text-sky-200">
          Patient: Demo Patient · Visit: 2026-09-18
        </div>
      </header>

      {/* Session Dashboard */}
      {session.isConnected && (
        <SessionDashboard
          activeTooth={session.activeTooth}
          activeSurface={session.activeSurface}
          pendingCount={session.pendingVersions.size}
          lastTranscript={session.lastTranscript}
          lastEventSource={session.lastEventSource}
          lastLatencyMs={session.lastLatencyMs}
          sessionStartTime={session.sessionStartTime}
          teeth={teeth}
        />
      )}

      {/* Main content */}
      <main className="flex-1 flex flex-col lg:flex-row gap-4 p-4 overflow-hidden">
        {/* Chart area */}
        <div className="flex-1 bg-white rounded-xl shadow-sm border border-slate-200 p-4 overflow-auto">
          <h2 className="text-slate-700 font-semibold text-sm mb-3 uppercase tracking-wide">Periodontal Chart</h2>
          <PeriodontalChart
            teeth={teeth}
            activeTooth={session.activeTooth}
            activeSurface={session.activeSurface}
          />
        </div>

        {/* Voice control sidebar */}
        <div className="lg:w-72 flex flex-col gap-3">
          <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-4">
            <VoiceControl
              onStart={handleStart}
              onStop={handleStop}
              isConnected={session.isConnected}
              isRecording={session.isRecording}
              isMockMode={session.isMockMode}
              isGroqEnabled={session.isGroqEnabled}
              activeTooth={session.activeTooth}
              activeSurface={session.activeSurface}
              sessionId={session.sessionId}
              lastEventSource={session.lastEventSource}
              lastLatencyMs={session.lastLatencyMs}
              analyserNode={analyserRef.current}
            />
          </div>

          {/* Legend */}
          <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-3">
            <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2">Pocket Depth</h3>
            <div className="space-y-1">
              {[
                { color: 'bg-green-500', label: '1–3mm · Healthy' },
                { color: 'bg-amber-500', label: '4–5mm · Moderate' },
                { color: 'bg-red-500',   label: '6mm · Deep' },
                { color: 'bg-red-900',   label: '7+mm · Critical' },
              ].map(({ color, label }) => (
                <div key={label} className="flex items-center gap-2 text-xs">
                  <div className={`w-3 h-3 rounded-full ${color}`} />
                  <span className="text-slate-600">{label}</span>
                </div>
              ))}
              <div className="flex items-center gap-2 text-xs mt-1">
                <div className="w-3 h-3 rounded-full bg-red-500 ring-2 ring-red-700" />
                <span className="text-slate-600">Bleeding on Probing</span>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <div className="w-3 h-3 rounded-full bg-purple-500" />
                <span className="text-slate-600">Suppuration</span>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <div className="w-3 h-3 rounded-full bg-yellow-500" />
                <span className="text-slate-600">Plaque</span>
              </div>
            </div>
          </div>

          {/* Voice commands */}
          <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-3">
            <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-2">Voice Commands</h3>
            <div className="space-y-0.5 text-xs text-slate-600 font-mono">
              {[
                '"tooth 14"',
                '"three two four"',
                '"bleeding" / "bop"',
                '"recession two"',
                '"furcation class two"',
                '"mobility one"',
                '"suppuration"',
                '"plaque positive"',
                '"next"',
                '"scratch that"',
                '"make site 2 a 5"',
              ].map(cmd => (
                <div key={cmd}>{cmd}</div>
              ))}
            </div>
          </div>
        </div>
      </main>

      {/* Live transcript strip */}
      <div className="border-t border-slate-200 flex-shrink-0">
        <LiveTranscript
          history={transcriptHistory}
          currentText={session.lastTranscript}
        />
      </div>
    </div>
  )
}
