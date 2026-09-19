import React from 'react'
import { ToothCell } from './ToothCell'
import type { ToothSurfaceData } from '../../lib/types'

interface ToothGridProps {
  toothNumbers: readonly number[]
  teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>
  activeTooth: number
  toothWidth?: number
}

export function ToothGrid({
  toothNumbers,
  teeth,
  activeTooth,
  toothWidth = 52,
}: ToothGridProps) {
  return (
    <div className="flex bg-slate-200 border-y border-slate-300">
      {toothNumbers.map(toothNum => (
        <ToothCell
          key={toothNum}
          toothNum={toothNum}
          buccalData={teeth[toothNum].buccal}
          lingualData={teeth[toothNum].lingual}
          isActive={toothNum === activeTooth}
          toothWidth={toothWidth}
        />
      ))}
    </div>
  )
}
