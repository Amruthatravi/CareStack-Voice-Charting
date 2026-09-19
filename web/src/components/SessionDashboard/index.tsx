import React, { useEffect, useState } from 'react'
import { Activity, Droplets, TrendingUp, Clock, Zap, Brain } from 'lucide-react'
import type { RouterSource, ToothData } from '../../lib/types'
import {
  calcBopPercent,
  calcMeanPD,
  countTeethCharted,
  countDeepSites,
} from '../../lib/chartHelpers'

interface SessionDashboardProps {
  activeTooth: number
  activeSurface: string
  pendingCount: number
  lastTranscript: string
  lastEventSource: RouterSource | null
  lastLatencyMs: number | null
  sessionStartTime: number | null
  teeth: Record<number, ToothData>
}

function useElapsed(startTime: number | null): string {
  const [elapsed, setElapsed] = useState('0:00')
  useEffect(() => {
    if (!startTime) return
    const id = setInterval(() => {
      const s = Math.floor((Date.now() - startTime) / 1000)
      setElapsed(`${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`)
    }, 1000)
    return () => clearInterval(id)
  }, [startTime])
  return elapsed
}

export function SessionDashboard({
  activeTooth,
  activeSurface,
  pendingCount,
  lastTranscript,
  lastEventSource,
  lastLatencyMs,
  sessionStartTime,
  teeth,
}: SessionDashboardProps) {
  const elapsed = useElapsed(sessionStartTime)
  const bop = calcBopPercent(teeth)
  const meanPD = calcMeanPD(teeth)
  const charted = countTeethCharted(teeth)
  const deepSites = countDeepSites(teeth)

  return (
    <div className="bg-sky-700 text-white px-4 py-2">
      <div className="flex items-center gap-4 flex-wrap">
        {/* Active tooth */}
        <div className="flex items-center gap-1.5 text-sm">
          <Activity size={14} className="text-sky-300" />
          <span className="text-sky-300">Active:</span>
          <span className="font-bold">#{activeTooth} {activeSurface}</span>
        </div>

        {/* Charted */}
        <div className="flex items-center gap-1.5 text-sm">
          <span className="text-sky-300">Charted:</span>
          <span className="font-bold">{charted}/32</span>
        </div>

        {/* BOP */}
        <div className="flex items-center gap-1.5 text-sm">
          <Droplets size={14} className="text-sky-300" />
          <span className="text-sky-300">BOP:</span>
          <span className={`font-bold ${
            bop >= 30 ? 'text-red-300' : bop >= 15 ? 'text-amber-300' : 'text-green-300'
          }`}>{bop}%</span>
        </div>

        {/* Mean PD */}
        <div className="flex items-center gap-1.5 text-sm">
          <TrendingUp size={14} className="text-sky-300" />
          <span className="text-sky-300">Mean PD:</span>
          <span className="font-bold">{meanPD}mm</span>
        </div>

        {/* Deep sites */}
        <div className="flex items-center gap-1.5 text-sm">
          <span className="text-sky-300">≥4mm:</span>
          <span className={`font-bold ${
            deepSites > 10 ? 'text-red-300' : deepSites > 3 ? 'text-amber-300' : 'text-green-300'
          }`}>{deepSites}</span>
        </div>

        {/* Router path */}
        {lastEventSource && (
          <div className="flex items-center gap-1 text-xs">
            {lastEventSource === 'regex'
              ? <><Zap size={11} className="text-green-300" /><span className="text-green-300">Regex</span></>
              : <><Brain size={11} className="text-purple-300" /><span className="text-purple-300">Groq AI</span></>
            }
            {lastLatencyMs !== null && (
              <span className="text-sky-400">{lastLatencyMs}ms</span>
            )}
          </div>
        )}

        {/* Pending */}
        {pendingCount > 0 && (
          <div className="flex items-center gap-1 text-xs text-amber-300">
            <div className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
            {pendingCount} pending
          </div>
        )}

        {/* Elapsed */}
        <div className="flex items-center gap-1.5 text-sm ml-auto">
          <Clock size={13} className="text-sky-300" />
          <span className="font-mono text-sky-200">{elapsed}</span>
        </div>
      </div>

      {/* Live transcript */}
      {lastTranscript && (
        <div className="text-xs text-sky-200 mt-1 truncate">
          <span className="text-sky-400">Last: </span>{lastTranscript}
        </div>
      )}
    </div>
  )
}
