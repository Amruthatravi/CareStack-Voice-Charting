export type Surface = 'buccal' | 'lingual'
export type PocketDepthReading = [number, number, number]
export type BleedingReading = [boolean, boolean, boolean]
export type RouterSource = 'regex' | 'groq'

export interface ToothSurfaceData {
  pocket_depth: PocketDepthReading | null   // [mesial, mid, distal] in mm
  bleeding: BleedingReading                  // per-site bleeding
  recession: number | null                   // mm
  furcation: 1 | 2 | 3 | null
  mobility: 0 | 1 | 2 | 3 | null
  suppuration: boolean
  plaque: boolean | null
  _optimistic: boolean                       // true = not yet DB confirmed
  _version: number
}

export interface ToothData {
  buccal: ToothSurfaceData
  lingual: ToothSurfaceData
}

export interface ChartEvent {
  session_id: string
  tenant_id: string
  patient_id: string
  event_type: 'measurement' | 'correction' | 'navigate' | 'transcript'
  tooth_num: number
  surface: Surface
  measurements: {
    pocket_depth?: PocketDepthReading
    bleeding?: boolean
    recession?: number
    furcation?: 1 | 2 | 3
    mobility?: 0 | 1 | 2 | 3
    suppuration?: boolean
    plaque?: boolean
    auto_advance?: boolean
    undo_last?: boolean
    site_correction?: { site_index: number; new_value: number }
    validation_warning?: string
    tooth?: number
  }
  confidence: number
  source_text: string
  is_partial: boolean
  version: number
  source: RouterSource
  timestamp: string
}

export interface SessionState {
  sessionId: string | null
  isConnected: boolean
  isRecording: boolean
  isMockMode: boolean
  isGroqEnabled: boolean
  activeTooth: number
  activeSurface: Surface
  lastTranscript: string
  lastVersion: number
  lastEventSource: RouterSource | null
  lastLatencyMs: number | null
  pendingVersions: Set<number>
  sessionStartTime: number | null
}

export interface TranscriptEntry {
  text: string
  timestamp: number
  eventType?: 'measurement' | 'correction' | 'navigate' | null
  source?: RouterSource | null
  isFinal: boolean
}

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error' | 'reconnecting'
