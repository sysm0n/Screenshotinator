// Backend → frontend event names. Keep in sync with internal/events/events.go.
export const Events = {
  ScreenshotAdded: 'screenshot:added',
  ScreenshotUpdated: 'screenshot:updated',
  SessionEnded: 'session:ended',
  WindowRestored: 'window:restored',
  HotkeyError: 'hotkey:error',
} as const

export type EventName = (typeof Events)[keyof typeof Events]
