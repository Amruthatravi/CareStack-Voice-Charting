import { create } from 'zustand'
import { temporal } from 'zundo'
import type { ChartEvent, RouterSource, SessionState, Surface, ToothData } from '../lib/types'
import { createEmptyTeeth, getNextTooth } from '../lib/chartHelpers'

interface ChartStore {
  // Chart data
  teeth: Record<number, ToothData>
  session: SessionState
  transcriptHistory: Array<{ text: string; timestamp: number; eventType?: string | null; source?: RouterSource | null; isFinal: boolean }>

  // Actions
  applyEvent: (event: ChartEvent, latencyMs?: number) => void
  confirmVersion: (version: number) => void
  rollbackVersion: (version: number) => void
  setSession: (updates: Partial<SessionState>) => void
  addTranscript: (text: string, eventType?: string | null, source?: RouterSource | null, isFinal?: boolean) => void
  advanceTooth: () => void
  reset: () => void
}

const initialSession: SessionState = {
  sessionId: null,
  isConnected: false,
  isRecording: false,
  isMockMode: false,
  isGroqEnabled: false,
  activeTooth: 1,
  activeSurface: 'buccal',
  lastTranscript: '',
  lastVersion: 0,
  lastEventSource: null,
  lastLatencyMs: null,
  pendingVersions: new Set(),
  sessionStartTime: null,
}

export const useChartStore = create<ChartStore>()(
  temporal(
    (set) => ({
      teeth: createEmptyTeeth(),
      session: initialSession,
      transcriptHistory: [],

      applyEvent: (event: ChartEvent, latencyMs?: number) => {
        set(state => {
          const newTeeth = { ...state.teeth }
          const newSession = { ...state.session }

          // Track router source + latency
          if (event.source) newSession.lastEventSource = event.source
          if (latencyMs !== undefined) newSession.lastLatencyMs = latencyMs

          if (event.event_type === 'navigate') {
            if (event.tooth_num && event.tooth_num > 0) {
              newSession.activeTooth = event.tooth_num
            }
            if (event.surface) {
              newSession.activeSurface = event.surface as Surface
            }
            if (event.measurements.tooth) {
              newSession.activeTooth = event.measurements.tooth
            }
            if (event.measurements.auto_advance) {
              const next = getNextTooth(newSession.activeTooth, newSession.activeSurface)
              newSession.activeTooth = next.tooth
              newSession.activeSurface = next.surface
            }
            return { teeth: newTeeth, session: newSession }
          }

          if (event.event_type === 'correction') {
            if (event.measurements.undo_last) {
              return state // Handled by zundo temporal undo
            }
            if (event.measurements.site_correction) {
              const { site_index, new_value } = event.measurements.site_correction
              const tooth = { ...newTeeth[event.tooth_num] }
              const surface = { ...tooth[event.surface as Surface] }
              if (surface.pocket_depth) {
                const pd = [...surface.pocket_depth] as [number, number, number]
                pd[site_index] = new_value
                surface.pocket_depth = pd
                surface._optimistic = true
                surface._version = event.version
              }
              tooth[event.surface as Surface] = surface
              newTeeth[event.tooth_num] = tooth
            }
            return { teeth: newTeeth, session: newSession }
          }

          if (event.event_type === 'measurement') {
            const tooth = { ...newTeeth[event.tooth_num] }
            const surface = { ...tooth[event.surface as Surface] }

            if (event.measurements.pocket_depth) {
              surface.pocket_depth = event.measurements.pocket_depth
            }
            if (event.measurements.bleeding !== undefined) {
              const b = event.measurements.bleeding
              surface.bleeding = [b, b, b]
            }
            if (event.measurements.recession !== undefined) {
              surface.recession = event.measurements.recession
            }
            if (event.measurements.furcation !== undefined) {
              surface.furcation = event.measurements.furcation as 1 | 2 | 3
            }
            if (event.measurements.mobility !== undefined) {
              surface.mobility = event.measurements.mobility as 0 | 1 | 2 | 3
            }
            if (event.measurements.suppuration !== undefined) {
              surface.suppuration = event.measurements.suppuration
            }
            if (event.measurements.plaque !== undefined) {
              surface.plaque = event.measurements.plaque
            }

            surface._optimistic = true
            surface._version = event.version
            tooth[event.surface as Surface] = surface
            newTeeth[event.tooth_num] = tooth

            // Track pending
            const pendingVersions = new Set(state.session.pendingVersions)
            pendingVersions.add(event.version)
            newSession.pendingVersions = pendingVersions
            newSession.lastVersion = event.version
          }

          return { teeth: newTeeth, session: newSession }
        })
      },

      confirmVersion: (version: number) => {
        set(state => {
          const newTeeth = { ...state.teeth }
          for (const toothNum in newTeeth) {
            const tooth = newTeeth[Number(toothNum)]
            for (const surf of ['buccal', 'lingual'] as Surface[]) {
              if (tooth[surf]._version === version && tooth[surf]._optimistic) {
                newTeeth[Number(toothNum)] = {
                  ...tooth,
                  [surf]: { ...tooth[surf], _optimistic: false },
                }
              }
            }
          }
          const pendingVersions = new Set(state.session.pendingVersions)
          pendingVersions.delete(version)
          return { teeth: newTeeth, session: { ...state.session, pendingVersions } }
        })
      },

      rollbackVersion: (version: number) => {
        useChartStore.temporal.getState().undo()
        set(state => {
          const pendingVersions = new Set(state.session.pendingVersions)
          pendingVersions.delete(version)
          return { session: { ...state.session, pendingVersions } }
        })
      },

      setSession: (updates) => set(state => ({ session: { ...state.session, ...updates } })),

      addTranscript: (text, eventType, source, isFinal = true) => set(state => ({
        transcriptHistory: [
          ...state.transcriptHistory.slice(-99),
          { text, timestamp: Date.now(), eventType: eventType ?? null, source: source ?? null, isFinal },
        ],
      })),

      advanceTooth: () => set(state => {
        const next = getNextTooth(state.session.activeTooth, state.session.activeSurface)
        return { session: { ...state.session, activeTooth: next.tooth, activeSurface: next.surface } }
      }),

      reset: () => set({
        teeth: createEmptyTeeth(),
        session: initialSession,
        transcriptHistory: [],
      }),
    }),
    { limit: 50 }  // 50-level undo history
  )
)
