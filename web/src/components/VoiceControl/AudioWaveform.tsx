import React, { useEffect, useRef } from 'react'

interface AudioWaveformProps {
  analyserNode: AnalyserNode | null
  isActive: boolean
}

export function AudioWaveform({ analyserNode, isActive }: AudioWaveformProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const W = canvas.width
    const H = canvas.height

    const draw = () => {
      rafRef.current = requestAnimationFrame(draw)

      ctx.clearRect(0, 0, W, H)

      if (!analyserNode || !isActive) {
        // Draw flat idle line
        ctx.strokeStyle = '#475569'
        ctx.lineWidth = 1.5
        ctx.beginPath()
        ctx.moveTo(0, H / 2)
        ctx.lineTo(W, H / 2)
        ctx.stroke()
        return
      }

      const bufferLength = analyserNode.frequencyBinCount
      const dataArray = new Uint8Array(bufferLength)
      analyserNode.getByteTimeDomainData(dataArray)

      // Compute RMS for VAD coloring
      let sum = 0
      for (let i = 0; i < bufferLength; i++) {
        const v = (dataArray[i] - 128) / 128
        sum += v * v
      }
      const rms = Math.sqrt(sum / bufferLength)
      const isSpeaking = rms > 0.01

      ctx.strokeStyle = isSpeaking ? '#22C55E' : '#64748B'
      ctx.lineWidth = 2
      ctx.beginPath()

      const sliceWidth = W / bufferLength
      let x = 0

      for (let i = 0; i < bufferLength; i++) {
        const v = dataArray[i] / 128.0
        const y = (v * H) / 2
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
        x += sliceWidth
      }

      ctx.lineTo(W, H / 2)
      ctx.stroke()
    }

    draw()
    return () => cancelAnimationFrame(rafRef.current)
  }, [analyserNode, isActive])

  return (
    <canvas
      ref={canvasRef}
      width={220}
      height={48}
      className="w-full rounded-lg bg-slate-900"
    />
  )
}
