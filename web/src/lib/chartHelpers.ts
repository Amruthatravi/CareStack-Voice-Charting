import type { PocketDepthReading, Surface, ToothSurfaceData } from './types'

export function getPocketDepthColor(mm: number): string {
  if (mm <= 3) return '#16A34A'   // green - healthy
  if (mm <= 5) return '#D97706'   // amber - moderate
  if (mm <= 6) return '#DC2626'   // red - deep
  return '#7F1D1D'                 // dark red - critical (7mm+)
}

export function getPocketDepthBg(mm: number): string {
  if (mm <= 3) return 'bg-green-100 text-green-800'
  if (mm <= 5) return 'bg-amber-100 text-amber-800'
  if (mm <= 6) return 'bg-red-100 text-red-800'
  return 'bg-red-900 text-red-100'
}

export function createEmptySurface(): ToothSurfaceData {
  return {
    pocket_depth: null,
    bleeding: [false, false, false],
    recession: null,
    furcation: null,
    mobility: null,
    suppuration: false,
    plaque: null,
    _optimistic: false,
    _version: 0,
  }
}

export function createEmptyTeeth(): Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }> {
  const teeth: Record<number, any> = {}
  for (let i = 1; i <= 32; i++) {
    teeth[i] = { buccal: createEmptySurface(), lingual: createEmptySurface() }
  }
  return teeth
}

export const UPPER_ARCH = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16] as const
export const LOWER_ARCH = [32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17] as const
const ARCH_ORDER = [...UPPER_ARCH, ...LOWER_ARCH] as number[]

export function maxPocketDepth(pd: PocketDepthReading | null): number {
  if (!pd) return 0
  return Math.max(...pd)
}

export function isClinicallySignificant(surface: ToothSurfaceData): boolean {
  const maxPD = maxPocketDepth(surface.pocket_depth)
  return maxPD >= 4 || surface.recession !== null || surface.furcation !== null
}

export function getFurcationSymbol(f: 1 | 2 | 3 | null): string {
  if (!f) return ''
  return ['I', 'II', 'III'][f - 1]
}

export function getNextTooth(current: number, surface: Surface): { tooth: number; surface: Surface } {
  const idx = ARCH_ORDER.indexOf(current)
  if (surface === 'buccal') {
    return { tooth: current, surface: 'lingual' }
  }
  // Move to next tooth buccal
  const nextIdx = (idx + 1) % ARCH_ORDER.length
  return { tooth: ARCH_ORDER[nextIdx], surface: 'buccal' }
}

export function toothLabel(n: number): string {
  const labels: Record<number, string> = {
    1: 'Upper Right Third Molar', 2: 'Upper Right Second Molar', 3: 'Upper Right First Molar',
    4: 'Upper Right Second Premolar', 5: 'Upper Right First Premolar', 6: 'Upper Right Canine',
    7: 'Upper Right Lateral Incisor', 8: 'Upper Right Central Incisor',
    9: 'Upper Left Central Incisor', 10: 'Upper Left Lateral Incisor', 11: 'Upper Left Canine',
    12: 'Upper Left First Premolar', 13: 'Upper Left Second Premolar', 14: 'Upper Left First Molar',
    15: 'Upper Left Second Molar', 16: 'Upper Left Third Molar',
    17: 'Lower Left Third Molar', 18: 'Lower Left Second Molar', 19: 'Lower Left First Molar',
    20: 'Lower Left Second Premolar', 21: 'Lower Left First Premolar', 22: 'Lower Left Canine',
    23: 'Lower Left Lateral Incisor', 24: 'Lower Left Central Incisor',
    25: 'Lower Right Central Incisor', 26: 'Lower Right Lateral Incisor', 27: 'Lower Right Canine',
    28: 'Lower Right First Premolar', 29: 'Lower Right Second Premolar', 30: 'Lower Right First Molar',
    31: 'Lower Right Second Molar', 32: 'Lower Right Third Molar',
  }
  return labels[n] ?? `Tooth ${n}`
}

export function calcBopPercent(teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>): number {
  let totalSites = 0
  let bleedingSites = 0
  for (const tooth of Object.values(teeth)) {
    for (const surf of [tooth.buccal, tooth.lingual]) {
      if (surf.pocket_depth) {
        totalSites += 3
        bleedingSites += surf.bleeding.filter(Boolean).length
      }
    }
  }
  return totalSites === 0 ? 0 : Math.round((bleedingSites / totalSites) * 100)
}

export function calcMeanPD(teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>): number {
  let total = 0
  let count = 0
  for (const tooth of Object.values(teeth)) {
    for (const surf of [tooth.buccal, tooth.lingual]) {
      if (surf.pocket_depth) {
        total += surf.pocket_depth.reduce((a, b) => a + b, 0)
        count += 3
      }
    }
  }
  return count === 0 ? 0 : Math.round((total / count) * 10) / 10
}

export function countTeethCharted(teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>): number {
  return Object.values(teeth).filter(t => t.buccal.pocket_depth || t.lingual.pocket_depth).length
}

export function countDeepSites(teeth: Record<number, { buccal: ToothSurfaceData; lingual: ToothSurfaceData }>): number {
  let count = 0
  for (const tooth of Object.values(teeth)) {
    for (const surf of [tooth.buccal, tooth.lingual]) {
      if (surf.pocket_depth) {
        count += surf.pocket_depth.filter(v => v >= 4).length
      }
    }
  }
  return count
}
