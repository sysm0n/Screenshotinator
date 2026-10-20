import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { Settings } from '../lib/types'

interface Props {
  onClose: () => void
  onSaved: () => void
  onError: (msg: string) => void
}

export default function SettingsDialog({ onClose, onSaved, onError }: Props) {
  const [s, setS] = useState<Settings | null>(null)
  const [showKey, setShowKey] = useState(false)

  useEffect(() => {
    api.getSettings().then(setS).catch((e) => onError(String(e)))
  }, [])

  if (!s) return null
  const field = 'w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 outline-none focus:border-indigo-500'

  const save = async () => {
    try {
      await api.saveSettings(s)
      onSaved()
      onClose()
    } catch (e) {
      onError(String(e))
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-6" onClick={onClose}>
      <div className="w-full max-w-lg space-y-4 rounded-2xl border border-slate-800 bg-slate-900 p-6" onClick={(e) => e.stopPropagation()}>
        <h2 className="text-lg font-semibold text-white">LLM settings</h2>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-400">Endpoint URL (OpenAI-compatible chat completions)</span>
          <input className={field} value={s.url} onChange={(e) => setS({ ...s, url: e.target.value })} placeholder="http://localhost:11434/v1/chat/completions" />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-400">Model ID</span>
          <input className={field} value={s.model} onChange={(e) => setS({ ...s, model: e.target.value })} placeholder="llama3.2-vision" />
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-400">API key (optional – leave empty for none)</span>
          <div className="flex gap-2">
            <input
              className={field}
              type={showKey ? 'text' : 'password'}
              value={s.apiKey}
              onChange={(e) => setS({ ...s, apiKey: e.target.value })}
              autoComplete="off"
            />
            <button type="button" onClick={() => setShowKey(!showKey)} className="rounded-lg border border-slate-700 px-3 text-xs text-slate-300 hover:bg-slate-800">
              {showKey ? 'Hide' : 'Show'}
            </button>
          </div>
        </label>

        <label className="block text-sm">
          <span className="mb-1 block text-slate-400">Restore shortcut (modifiers ctrl, shift, alt, cmd + one key, e.g. ctrl+shift+e)</span>
          <input className={field} value={s.hotkey} onChange={(e) => setS({ ...s, hotkey: e.target.value })} placeholder="ctrl+shift+e" />
        </label>

        <div className="flex justify-end gap-3 pt-2">
          <button onClick={onClose} className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800">Cancel</button>
          <button onClick={save} className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500">Save</button>
        </div>
      </div>
    </div>
  )
}
