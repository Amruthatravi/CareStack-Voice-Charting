import React from 'react'
import { UPPER_ARCH, LOWER_ARCH } from '../../lib/chartHelpers'
import type { ToothSurfaceData } from '../../lib/types'
import { PocketDepthGraph } from './PocketDepthGraph'
import { BleedingDots } from './BleedingDots'
import { RecessionBar } from './RecessionBar'
import { ToothGrid } from './ToothGrid'

interface PeriodontalChartProps {
  teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>
  activeTooth: number
  activeSurface: 'buccal' | 'lingual'
}

export function PeriodontalChart({ teeth, activeTooth }: PeriodontalChartProps) {
  const toothWidth = 52

  const renderArch = (title: string, toothNumbers: readonly number[]) => (
    <div className="mb-8 last:mb-0">
      <h3 className="text-sm font-bold text-slate-600 mb-2">{title}</h3>
      <div className="inline-block border border-slate-300 rounded overflow-hidden shadow-sm bg-white">
        {/* Buccal */}
        <PocketDepthGraph toothNumbers={toothNumbers} surface="buccal" teeth={teeth} toothWidth={toothWidth} />
        <BleedingDots toothNumbers={toothNumbers} surface="buccal" teeth={teeth} toothWidth={toothWidth} />
        <RecessionBar toothNumbers={toothNumbers} surface="buccal" teeth={teeth} toothWidth={toothWidth} />
        
        {/* Tooth Grid */}
        <ToothGrid toothNumbers={toothNumbers} teeth={teeth} activeTooth={activeTooth} toothWidth={toothWidth} />
        
        {/* Lingual (Inverted order of rows) */}
        <RecessionBar toothNumbers={toothNumbers} surface="lingual" teeth={teeth} toothWidth={toothWidth} />
        <BleedingDots toothNumbers={toothNumbers} surface="lingual" teeth={teeth} toothWidth={toothWidth} />
        <PocketDepthGraph toothNumbers={toothNumbers} surface="lingual" teeth={teeth} inverted toothWidth={toothWidth} />
      </div>
    </div>
  )

  return (
    <div className="overflow-x-auto pb-4">
      <div className="min-w-max">
        {renderArch('Upper Arch (1-16)', UPPER_ARCH)}
        {renderArch('Lower Arch (32-17)', LOWER_ARCH)}
      </div>
    </div>
  )
}
