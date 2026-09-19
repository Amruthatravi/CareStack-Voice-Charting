import { useCallback } from 'react'
import { useChartStore } from '../store/chartStore'
import type { ChartEvent, RouterSource } from '../lib/types'

export function useChartWebSocket() {
  const { applyEvent, confirmVersion, rollbackVersion, setSession, addTranscript } = useChartStore()

  const handleMessage = useCallback((data: any) => {
    switch (data.type) {
      case 'session_ready':
        setSession({
          sessionId: data.session_id,
          isMockMode: data.mock_mode,
          isGroqEnabled: data.groq_enabled ?? false,
          activeTooth: data.active_tooth ?? 1,
          activeSurface: data.active_surface ?? 'buccal',
          sessionStartTime: Date.now(),
        })
        break

      case 'transcript':
        if (data.is_final) {
          addTranscript(data.text, null, null, true)
          setSession({ lastTranscript: data.text })
        } else {
          addTranscript(data.text, null, null, false)
        }
        break

      case 'chart_event': {
        const event = data.event as ChartEvent
        const source = (data.source ?? event.source ?? 'regex') as RouterSource
        const latencyMs = data.latency_ms as number | undefined

        // Handle undo
        if (event.event_type === 'correction' && event.measurements.undo_last) {
          useChartStore.temporal.getState().undo()
        } else {
          applyEvent({ ...event, source }, latencyMs)
        }

        // Update active tooth/surface
        if (event.event_type === 'navigate' || event.event_type === 'measurement') {
          if (!event.measurements.auto_advance) {
            setSession({
              activeTooth: event.tooth_num || useChartStore.getState().session.activeTooth,
              activeSurface: (event.surface as any) || useChartStore.getState().session.activeSurface,
            })
          }
        }

        // Add to transcript history with event metadata
        const lastTranscript = useChartStore.getState().session.lastTranscript
        if (lastTranscript && event.event_type !== 'navigate') {
          addTranscript(lastTranscript, event.event_type, source, true)
        }

        break
      }

      case 'db_confirmed':
        confirmVersion(data.version)
        break

      case 'db_failed':
        rollbackVersion(data.version)
        break

      case 'error':
        console.error('Server error:', data.message)
        break
    }
  }, [applyEvent, confirmVersion, rollbackVersion, setSession, addTranscript])

  return { handleMessage }
}
