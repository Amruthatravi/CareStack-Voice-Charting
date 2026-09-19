/**
 * DentalAudioProcessor — AudioWorklet
 * Frame: 20ms at 16kHz = 320 samples
 * VAD:   RMS energy threshold at 0.005
 */
class DentalAudioProcessor extends AudioWorkletProcessor {
  constructor() {
    super()
    this.FRAME_SIZE = 320       // 20ms @ 16kHz
    this.VAD_THRESHOLD = 0.005  // RMS silence gate
    this._buffer = new Float32Array(this.FRAME_SIZE)
    this._bufferIndex = 0
    this._vadActive = false
  }

  _computeRMS(samples) {
    let sum = 0
    for (let i = 0; i < samples.length; i++) {
      sum += samples[i] * samples[i]
    }
    return Math.sqrt(sum / samples.length)
  }

  process(inputs, outputs, parameters) {
    const input = inputs[0]
    if (!input || input.length === 0) return true

    const channelData = input[0]
    for (let i = 0; i < channelData.length; i++) {
      this._buffer[this._bufferIndex++] = channelData[i]

      if (this._bufferIndex >= this.FRAME_SIZE) {
        const rms = this._computeRMS(this._buffer)
        const isSpeaking = rms >= this.VAD_THRESHOLD

        // Notify main thread when VAD state changes (for waveform UI coloring)
        if (isSpeaking !== this._vadActive) {
          this._vadActive = isSpeaking
          this.port.postMessage({ type: 'vad', active: isSpeaking })
        }

        // Convert Float32 [-1,1] to Int16 PCM
        const pcm16 = new Int16Array(this.FRAME_SIZE)
        for (let j = 0; j < this.FRAME_SIZE; j++) {
          const s = Math.max(-1, Math.min(1, this._buffer[j]))
          pcm16[j] = s < 0 ? s * 0x8000 : s * 0x7FFF
        }

        this.port.postMessage({ pcm16: pcm16.buffer }, [pcm16.buffer])
        this._bufferIndex = 0
      }
    }
    return true
  }
}

registerProcessor('dental-audio-processor', DentalAudioProcessor)

