import React from 'react'
import type { ToothSurfaceData } from '../../lib/types'
import { getFurcationSymbol } from '../../lib/chartHelpers'

interface ToothCellProps {
  toothNum: number
  buccalData: ToothSurfaceData
  lingualData: ToothSurfaceData
  isActive: boolean
  toothWidth?: number
}

export function ToothCell({
  toothNum,
  buccalData,
  lingualData,
  isActive,
  toothWidth = 52,
}: ToothCellProps) {
  const isOptimistic = buccalData._optimistic || lingualData._optimistic
  const furcation = buccalData.furcation || lingualData.furcation
  const mobility = buccalData.mobility !== null ? buccalData.mobility : lingualData.mobility
  const hasSuppuration = buccalData.suppuration || lingualData.suppuration
  const hasPlaque = buccalData.plaque || lingualData.plaque

  return (
    <div
      style={{ width: toothWidth }}
      className={`h-10 flex flex-col items-center justify-center relative border-r border-slate-200 last:border-r-0 select-none ${
        isActive ? 'bg-sky-100 border-b-2 border-b-sky-600' : 'bg-white border-b-2 border-b-transparent'
      }`}
    >
      <div className={`text-sm leading-none ${
        isActive ? 'font-bold text-sky-800' : 'font-medium text-slate-700'
      }`}>
        {toothNum}
        {mobility !== null && (
          <sup className="text-[9px] text-amber-700 font-bold ml-0.5">M{mobility}</sup>
        )}
      </div>

      {furcation !== null && (
        <div className="text-[10px] text-red-600 font-bold leading-none">
          {getFurcationSymbol(furcation)}
        </div>
      )}

      {/* Status indicators — top-right corner cluster */}
      <div className="absolute top-0.5 right-0.5 flex flex-col gap-0.5">
        {isOptimistic && (
          <div className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" title="Pending DB confirmation" />
        )}
        {hasSuppuration && (
          <div className="w-1.5 h-1.5 rounded-full bg-purple-500" title="Suppuration" />
        )}
        {hasPlaque && (
          <div className="w-1.5 h-1.5 rounded-full bg-yellow-500" title="Plaque" />
        )}
      </div>
    </div>
  )
}
