import React from 'react'
import { getPocketDepthColor } from '../../lib/chartHelpers'
import type { ToothSurfaceData } from '../../lib/types'

interface PocketDepthGraphProps {
  toothNumbers: readonly number[]
  surface: 'buccal' | 'lingual'
  teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>
  inverted?: boolean
  height?: number
  toothWidth?: number
}

export function PocketDepthGraph({
  toothNumbers,
  surface,
  teeth,
  inverted = false,
  height = 80,
  toothWidth = 52,
}: PocketDepthGraphProps) {
  const width = toothNumbers.length * toothWidth
  const MAX_MM = 12
  const scale = (height - 6) / MAX_MM

  const getPoints = (toothIndex: number, pd: [number, number, number]) => {
    const baseX = toothIndex * toothWidth
    return [
      { x: baseX + 6, y: inverted ? pd[0] * scale + 3 : height - pd[0] * scale - 3, val: pd[0] },
      { x: baseX + toothWidth / 2, y: inverted ? pd[1] * scale + 3 : height - pd[1] * scale - 3, val: pd[1] },
      { x: baseX + toothWidth - 6, y: inverted ? pd[2] * scale + 3 : height - pd[2] * scale - 3, val: pd[2] },
    ]
  }

  return (
    <div style={{ width, height }} className="relative bg-white border-b border-slate-200">
      <svg width={width} height={height} className="absolute inset-0 pointer-events-none">
        {/* Reference lines at 4mm and 6mm */}
        {[4, 6].map(mm => {
          const y = inverted ? mm * scale + 3 : height - mm * scale - 3
          return (
            <line
              key={mm}
              x1={0} y1={y} x2={width} y2={y}
              stroke={mm === 4 ? '#FEF3C7' : '#FEE2E2'}
              strokeWidth="1"
              strokeDasharray="2 4"
            />
          )
        })}

        {/* Baseline */}
        <line
          x1={0} y1={inverted ? 2 : height - 2}
          x2={width} y2={inverted ? 2 : height - 2}
          stroke="#E2E8F0" strokeWidth="1"
        />

        {toothNumbers.map((toothNum, index) => {
          const surfData = teeth[toothNum]?.[surface]
          if (!surfData?.pocket_depth) return null

          const pts = getPoints(index, surfData.pocket_depth)
          const maxPD = Math.max(...surfData.pocket_depth)
          const color = getPocketDepthColor(maxPD)
          const isOptimistic = surfData._optimistic
          const pointsStr = pts.map(p => `${p.x},${p.y}`).join(' ')

          // Inter-tooth dashed connector
          let nextLine = null
          if (index < toothNumbers.length - 1) {
            const nextTooth = toothNumbers[index + 1]
            const nextSurf = teeth[nextTooth]?.[surface]
            if (nextSurf?.pocket_depth) {
              const nextPts = getPoints(index + 1, nextSurf.pocket_depth)
              nextLine = (
                <line
                  x1={pts[2].x} y1={pts[2].y}
                  x2={nextPts[0].x} y2={nextPts[0].y}
                  stroke={color} strokeWidth="1.5"
                  strokeDasharray="3 3" opacity={0.45}
                />
              )
            }
          }

          return (
            <g key={toothNum}>
              <polyline points={pointsStr} fill="none" stroke={color} strokeWidth="2.5" strokeLinejoin="round" />
              {nextLine}
              {pts.map((p, i) => (
                <g key={i}>
                  {/* Optimistic shimmer ring */}
                  {isOptimistic && (
                    <circle cx={p.x} cy={p.y} r={6} fill="none" stroke="#F59E0B" strokeWidth="1.5" opacity={0.6}
                      style={{ animation: 'pulse 1.5s ease-in-out infinite' }}
                    />
                  )}
                  <circle cx={p.x} cy={p.y} r={3.5} fill={color} />
                  <text
                    x={p.x}
                    y={inverted ? p.y + 13 : p.y - 6}
                    fontSize="10"
                    fill={color}
                    textAnchor="middle"
                    fontWeight="bold"
                  >
                    {p.val}
                  </text>
                </g>
              ))}
            </g>
          )
        })}
      </svg>
    </div>
  )
}
