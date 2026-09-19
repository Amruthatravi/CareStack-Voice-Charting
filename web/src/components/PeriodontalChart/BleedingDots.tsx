import React from 'react'
import type { ToothSurfaceData } from '../../lib/types'

interface BleedingDotsProps {
  toothNumbers: readonly number[]
  surface: 'buccal' | 'lingual'
  teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>
  toothWidth?: number
}

export function BleedingDots({
  toothNumbers,
  surface,
  teeth,
  toothWidth = 52,
}: BleedingDotsProps) {
  return (
    <div className="flex border-b border-slate-200 bg-white" style={{ height: 16 }}>
      {toothNumbers.map(toothNum => {
        const surfData = teeth[toothNum]?.[surface]
        const bleeding = surfData?.bleeding ?? [false, false, false]
        // 3 sites: mesial, mid, distal
        const positions = [8, toothWidth / 2, toothWidth - 8]
        return (
          <div
            key={toothNum}
            className="relative flex-shrink-0 border-r border-slate-200 last:border-r-0"
            style={{ width: toothWidth, height: 16 }}
          >
            {bleeding.map((bleeds, i) => (
              <div
                key={i}
                style={{ left: positions[i] - 4, top: 4 }}
                className={`absolute w-2 h-2 rounded-full transition-all duration-300 ${
                  bleeds
                    ? 'bg-red-500 shadow-[0_0_4px_rgba(239,68,68,0.6)]'
                    : surfData?.pocket_depth ? 'bg-slate-200' : 'bg-transparent'
                }`}
              />
            ))}
          </div>
        )
      })}
    </div>
  )
}
