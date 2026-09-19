import React from 'react'
import type { ToothSurfaceData } from '../../lib/types'

interface RecessionBarProps {
  toothNumbers: readonly number[]
  surface: 'buccal' | 'lingual'
  teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>
  toothWidth?: number
}

export function RecessionBar({
  toothNumbers,
  surface,
  teeth,
  toothWidth = 52,
}: RecessionBarProps) {
  return (
    <div className="flex h-6 bg-slate-50 border-b border-slate-200">
      {toothNumbers.map(toothNum => {
        const data = teeth[toothNum]?.[surface]
        const rec = data?.recession
        
        let textColor = 'text-slate-400'
        if (rec !== null && rec > 0) {
          textColor = rec > 3 ? 'text-red-600 font-bold' : 'text-amber-600 font-bold'
        }

        return (
          <div
            key={toothNum}
            style={{ width: toothWidth }}
            className={`flex items-center justify-center text-xs ${textColor}`}
          >
            {rec !== null && rec > 0 ? rec : ''}
          </div>
        )
      })}
    </div>
  )
}
