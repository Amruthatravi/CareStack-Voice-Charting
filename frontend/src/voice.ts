// On-device speech recognition (Vosk WebAssembly). Audio never leaves the browser;
// only the recognised text is sent to the backend.
import { createModel, type Model } from "vosk-browser";

export const MODEL_URL = "/models/vosk-model-small-en-us-0.15.tar.gz";

// Closed grammar: a word outside this list cannot be transcribed.
const WORDS = [
  "zero one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen",
  "seventeen eighteen nineteen twenty thirty",
  "tooth number jump go to next missing implant extracted clear reset",
  "bleeding bleed bop suppuration plaque positive recession mobility furcation class",
  "buccal lingual facial palatal make change set site last that a scratch undo",
  "no wait actually sorry i mean was should be it and on",
].join(" ").split(" ");

export const GRAMMAR = JSON.stringify([...WORDS, "[unk]"]);

export interface VoiceHandlers {
  onPartial(text: string): void;
  onFinal(text: string): void;
}

export class VoiceEngine {
  private model: Model | null = null;
  private ctx: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private node: ScriptProcessorNode | null = null;
  private source: MediaStreamAudioSourceNode | null = null;

  async start(h: VoiceHandlers, onLoading?: () => void): Promise<void> {
    if (!this.model) {
      onLoading?.();
      this.model = await createModel(MODEL_URL);
    }
    this.stream = await navigator.mediaDevices.getUserMedia({
      video: false,
      audio: { echoCancellation: true, noiseSuppression: true, channelCount: 1 },
    });
    this.ctx = new AudioContext();
    const rec = new this.model.KaldiRecognizer(this.ctx.sampleRate, GRAMMAR);
    rec.on("result", (m) => {
      const text = ((m as any).result?.text ?? "").trim();
      if (text && text !== "[unk]") h.onFinal(text);
      h.onPartial("");
    });
    rec.on("partialresult", (m) => h.onPartial((m as any).result?.partial ?? ""));

    this.source = this.ctx.createMediaStreamSource(this.stream);
    this.node = this.ctx.createScriptProcessor(4096, 1, 1);
    this.node.onaudioprocess = (e) => {
      try {
        rec.acceptWaveform(e.inputBuffer);
      } catch {
        /* recognizer closed */
      }
    };
    this.source.connect(this.node);
    this.node.connect(this.ctx.destination);
  }

  stop() {
    this.node?.disconnect();
    this.source?.disconnect();
    this.stream?.getTracks().forEach((t) => t.stop());
    void this.ctx?.close();
    this.node = this.source = this.stream = this.ctx = null;
  }
}
