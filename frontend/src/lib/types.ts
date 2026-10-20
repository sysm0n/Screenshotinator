export interface Screenshot {
  id: number
  sessionId: string
  timestamp: string
  appName: string
  imagePath: string
  userNotes: string
  aiSummary: string
  analyzing: boolean
  aiError?: string // last analysis failure; runtime only
}

export interface SessionResult {
  sessionId: string
  screenshots: Screenshot[]
}

// One curated step sent for export, in final order.
export interface FrontendScreenshotData {
  id: number
  userNotes: string
  aiSummary: string
}

export interface Settings {
  url: string
  apiKey: string
  model: string
  hotkey: string
}
