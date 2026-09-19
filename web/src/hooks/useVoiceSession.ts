import { useRef, useCallback } from 'react'
import { useChartStore } from '../store/chartStore'

const WS_URL = '/ws/stream'
const DEMO_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJjbGluaWNpYW4tMDAxIiwidGVuYW50X2lkIjoiZGVtby1wcmFjdGljZSIsInBhdGllbnRfaWQiOiJwYXRpZW50LTAxIiwidmlzaXRfaWQiOiJ2aXNpdC0wMSIsInJvbGUiOiJwcmFjdGl0aW9uZXIiLCJleHAiOjk5OTk5OTk5OTl9.zUFYYv5uBKbDoB87u3IvaZMar95xsT1AeniI9_XY9v4'

export function useVoiceSession() {
  const audioCtxRef = useRef<AudioContext | null>(null)
  const workletNodeRef = useRef<AudioWorkletNode | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const mediaStreamRef = useRef<MediaStream | null>(null)
  const sessionActiveRef = useRef(false)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const onMessageRef = useRef<(data: any) => void>(() => undefined)
  const setSession = useChartStore(s => s.setSession)

  const startSession = useCallback(async (onMessage: (data: any) => void) => {
    sessionActiveRef.current = true
    onMessageRef.current = onMessage

    try {
      setSession({ isConnected: false })

      const ws = new WebSocket(`${WS_URL}?token=${DEMO_TOKEN}`)
      ws.binaryType = 'arraybuffer'
      wsRef.current = ws

      ws.onopen = async () => {
        setSession({ isConnected: true })

        // Request microphone with fallback to mock-only
        if (!mediaStreamRef.current) {
          try {
            mediaStreamRef.current = await navigator.mediaDevices.getUserMedia({
              audio: {
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true,
                channelCount: 1,
              },
            })
          } catch {
            // No mic — server will use mock mode, WS stays open
            setSession({ isRecording: true })
            return
          }
        }

        if (!sessionActiveRef.current) return

        if (audioCtxRef.current) {
          setSession({ isRecording: true })
          return
        }

        const audioCtx = new AudioContext({ sampleRate: 16000 })
        audioCtxRef.current = audioCtx

        // Create analyser for waveform visualization
        const analyser = audioCtx.createAnalyser()
        analyser.fftSize = 256
        analyserRef.current = analyser

        await audioCtx.audioWorklet.addModule('/worklets/dental-audio-processor.js')

        const source = audioCtx.createMediaStreamSource(mediaStreamRef.current)
        const workletNode = new AudioWorkletNode(audioCtx, 'dental-audio-processor', {
          numberOfOutputs: 0,
          processorOptions: { sampleRate: audioCtx.sampleRate },
        })
        workletNodeRef.current = workletNode

        // VAD messages from worklet
        workletNode.port.onmessage = ({ data }: MessageEvent) => {
          const currentWs = wsRef.current
          if (data.pcm16 && currentWs?.readyState === WebSocket.OPEN) {
            currentWs.send(data.pcm16)
          }
        }

        source.connect(analyser)
        source.connect(workletNode)
        setSession({ isRecording: true })
      }

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data as string)
          onMessageRef.current(data)
        } catch {
          // binary frame
        }
      }

      ws.onclose = () => {
        if (wsRef.current !== ws) return
        setSession({ isConnected: false })

        // Keep the microphone active until the user clicks stop.
        if (sessionActiveRef.current && reconnectTimerRef.current === null) {
          reconnectTimerRef.current = setTimeout(() => {
            reconnectTimerRef.current = null
            void startSession(onMessageRef.current)
          }, 1000)
        }
      }
      ws.onerror = () => setSession({ isConnected: false })
    } catch (err) {
      console.error('Voice session error:', err)
      setSession({ isConnected: false })
    }
  }, [setSession])

  const stopSession = useCallback(() => {
    sessionActiveRef.current = false
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    wsRef.current?.close()
    audioCtxRef.current?.close()
    workletNodeRef.current?.disconnect()
    mediaStreamRef.current?.getTracks().forEach(track => track.stop())
    wsRef.current = null
    audioCtxRef.current = null
    workletNodeRef.current = null
    mediaStreamRef.current = null
    analyserRef.current = null
    setSession({ isConnected: false, isRecording: false, sessionId: null, sessionStartTime: null })
  }, [setSession])

  return { startSession, stopSession, analyserRef }
}
