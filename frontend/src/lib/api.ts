// Thin typed wrapper over the Wails-generated bindings (frontend/wailsjs is created by `wails dev|build`).
import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { EventName } from './events'
import type { FrontendScreenshotData, Screenshot, SessionResult, Settings } from './types'

export const api = {
  hotkeyLabel: () => App.HotkeyLabel(),
  startSession: () => App.StartSession(),
  endSession: () => App.EndSession() as unknown as Promise<SessionResult>,
  getSession: (id: string) => App.GetSession(id) as unknown as Promise<Screenshot[]>,
  deleteScreenshot: (id: number) => App.DeleteScreenshot(id),
  updateNotes: (id: number, notes: string) => App.UpdateNotes(id, notes),
  analyze: (id: number, notes: string, global: string) => App.AnalyzeScreenshot(id, notes, global),
  getSettings: () => App.GetSettings() as unknown as Promise<Settings>,
  saveSettings: (s: Settings) => App.SaveSettings(s as never),
  getImage: (id: number) => App.GetImage(id),
  chooseExportFolder: () => App.ChooseExportFolder(),
  exportMarkdown: (sessionId: string, items: FrontendScreenshotData[], folder: string) =>
    App.ExportMarkdownDoc(sessionId, items as never, folder),
}

export const onEvent = <T,>(name: EventName, cb: (data: T) => void): (() => void) => EventsOn(name, cb)
