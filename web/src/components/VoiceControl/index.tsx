import React from 'react'
import { Mic, MicOff, Radio, Zap, Brain } from 'lucide-react'
import { AudioWaveform } from './AudioWaveform'
import type { RouterSource } from '../../lib/types'

interface VoiceControlProps {
  onStart: () => void
  onStop: () => void
  isConnected: boolean
  isRecording: boolean
  isMockMode: boolean
  isGroqEnabled: boolean
  activeTooth: number
  activeSurface: string
  sessionId: string | null
  lastEventSource: RouterSource | null
  lastLatencyMs: number | null
  analyserNode: AnalyserNode | null
}

export function VoiceControl({
  onStart,
  onStop,
  isConnected,
  isRecording,
  isMockMode,
  isGroqEnabled,
  activeTooth,
  activeSurface,
  sessionId,
  lastEventSource,
  lastLatencyMs,
  analyserNode,
}: VoiceControlProps) {
  const isRegex = lastEventSource === 'regex'

  return (
    <div className="flex flex-col items-center gap-3">
      {/* Mic Button */}
      <button
        onClick={isRecording ? onStop : onStart}
        className={`w-20 h-20 rounded-full flex items-center justify-center transition-all duration-300 shadow-md ${
          isRecording
            ? 'bg-red-100 text-red-600 animate-[pulse_2s_ease-in-out_infinite] border-4 border-red-300'
            : 'bg-slate-100 text-slate-500 hover:bg-sky-50 hover:text-sky-600 border-4 border-transparent'
        }`}
      >
        {isRecording ? <Mic size={36} /> : <MicOff size={36} />}
      </button>

      {/* Waveform */}
      <div className="w-full">
        <AudioWaveform analyserNode={analyserNode} isActive={isRecording} />
      </div>

      <div className="text-center w-full">
        <h3 className="font-semibold text-slate-800 mb-1 text-sm">Voice Input</h3>

        {/* Status Badge */}
        <div className="flex items-center justify-center gap-2 mb-2">
          <div
            className={`w-2 h-2 rounded-full ${
              isConnected
                ? isMockMode ? 'bg-amber-500' : 'bg-green-500'
                : 'bg-slate-400'
            }`}
          />
          <span className="text-xs text-slate-600 font-medium">
            {isConnected
              ? isMockMode ? 'Connected (Demo)' : 'Connected (LIVE)'
              : 'Disconnected'}
          </span>
          {isGroqEnabled && (
            <span className="text-xs bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded font-semibold">
              AI
            </span>
          )}
        </div>

        {/* Active Context */}
        <div className="bg-sky-50 rounded-lg p-2 border border-sky-100 flex items-center justify-center gap-2 mb-2">
          <Radio size={14} className="text-sky-600" />
          <span className="text-sm font-semibold text-sky-800">
            Tooth {activeTooth} — {activeSurface}
          </span>
        </div>

        {/* Router Path Badge */}
        {lastEventSource && (
          <div className={`flex items-center justify-center gap-1.5 text-xs px-2 py-1 rounded-full mb-2 ${
            isRegex
              ? 'bg-green-100 text-green-800'
              : 'bg-purple-100 text-purple-800'
          }`}>
            {isRegex
              ? <><Zap size={11} /> Regex Fast-Path</>
              : <><Brain size={11} /> Groq AI Slow-Path</>
            }
            {lastLatencyMs !== null && (
              <span className="opacity-70 ml-1">{lastLatencyMs}ms</span>
            )}
          </div>
        )}

        {/* Session ID */}
        {sessionId && (
          <div className="text-[10px] text-slate-400 font-mono">
            Session: {sessionId.split('-')[0]}
          </div>
        )}
      </div>

      {isMockMode && (
        <div className="w-full bg-amber-50 border border-amber-200 text-amber-800 text-xs p-2 rounded text-center">
          <strong>Demo Mode</strong>
          <p className="mt-0.5">Mock dental utterances play automatically.</p>
        </div>
      )}
    </div>
  )
}
