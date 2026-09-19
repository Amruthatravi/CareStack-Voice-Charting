import React, { useEffect, useRef } from 'react'
import type { RouterSource } from '../../lib/types'

interface TranscriptEntry {
  text: string
  timestamp: number
  eventType?: string | null
  source?: RouterSource | null
  isFinal: boolean
}

interface LiveTranscriptProps {
  history: TranscriptEntry[]
  currentText: string
}

function eventBadge(eventType?: string | null, source?: RouterSource | null) {
  if (!eventType) return null
  const colors: Record<string, string> = {
    measurement: 'bg-blue-800 text-blue-100',
    correction: 'bg-orange-700 text-orange-100',
    navigate: 'bg-green-800 text-green-100',
  }
  const cls = colors[eventType] ?? 'bg-slate-700 text-slate-200'
  const srcLabel = source === 'groq' ? ' 🧠' : source === 'regex' ? ' ⚡' : ''
  return (
    <span className={`inline-block text-[9px] px-1 py-0.5 rounded font-bold ml-1 ${cls}`}>
      {eventType}{srcLabel}
    </span>
  )
}

export function LiveTranscript({ history, currentText }: LiveTranscriptProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [history, currentText])

  const finalHistory = history.filter(h => h.isFinal).slice(-5)

  return (
    <div className="bg-slate-900 text-slate-300 h-24 p-2 font-mono text-xs overflow-y-auto" ref={scrollRef}>
      {finalHistory.length === 0 && !currentText && (
        <div className="opacity-30 italic">Waiting for speech...</div>
      )}
      {finalHistory.map((item, i) => (
        <div key={item.timestamp + i} className="opacity-60 flex items-baseline gap-1 mb-0.5">
          <span>{item.text}</span>
          {eventBadge(item.eventType, item.source)}
        </div>
      ))}
      {currentText && (
        <div className="text-white font-medium flex items-baseline gap-1">
          <span className="opacity-70 text-[10px]">▶</span>
          <span>{currentText}</span>
        </div>
      )}
    </div>
  )
}
