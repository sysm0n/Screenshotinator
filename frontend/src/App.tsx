import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, onEvent } from './lib/api'
import { Events } from './lib/events'
import type { FrontendScreenshotData, Screenshot, SessionResult } from './lib/types'
import ScreenshotCard from './components/ScreenshotCard'
import SettingsDialog from './components/SettingsDialog'
import RecordingPanel from './components/RecordingPanel'

type Phase = 'idle' | 'recording' | 'review'

export default function App() {
  const [phase, setPhase] = useState<Phase>('idle')
  const [sessionId, setSessionId] = useState('')
  const [shots, setShots] = useState<Screenshot[]>([])
  const [recCount, setRecCount] = useState(0)
  const [globalPrompt, setGlobalPrompt] = useState('')
  const [showSettings, setShowSettings] = useState(false)
  const [busy, setBusy] = useState(false)
  const [hotkey, setHotkey] = useState('Ctrl+Alt+E')
  const [toast, setToast] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null)

  const notify = (kind: 'ok' | 'err', text: string) => {
    setToast({ kind, text })
    window.setTimeout(() => setToast(null), 5000)
  }

  useEffect(() => {
    api.hotkeyLabel().then(setHotkey).catch(() => {})
    const offs = [
      onEvent<Screenshot>(Events.ScreenshotAdded, (s) => {
        setRecCount((c) => c + 1)
        setShots((prev) => (prev.some((p) => p.id === s.id) ? prev : [...prev, s]))
      }),
      onEvent<Screenshot>(Events.ScreenshotUpdated, (s) =>
        // keep local (possibly unsaved) notes, take AI fields from the backend
        setShots((prev) =>
          prev.map((p) => (p.id === s.id ? { ...p, aiSummary: s.aiSummary, analyzing: s.analyzing, aiError: s.aiError } : p)),
        ),
      ),
      // the restore shortcut stops recording on the Go side and hands us the finished session
      onEvent<SessionResult>(Events.SessionEnded, (r) => {
        setSessionId(r.sessionId)
        setShots(r.screenshots ?? [])
        setPhase('review')
      }),
      onEvent<string>(Events.HotkeyError, (m) => notify('err', `Global shortcut unavailable: ${m}`)),
    ]
    return () => offs.forEach((off) => off())
  }, [])

  const start = async () => {
    try {
      const id = await api.startSession()
      setSessionId(id)
      setShots([])
      setRecCount(0)
      setGlobalPrompt('')
      setPhase('recording') // the Go side hides the window right now
    } catch (e) {
      notify('err', String(e))
    }
  }

  const end = async () => {
    setBusy(true)
    try {
      const res = await api.endSession()
      setSessionId(res.sessionId)
      setShots(res.screenshots ?? [])
      setPhase('review')
    } catch (e) {
      // already ended through the shortcut: the session:ended event delivers the result
      if (!String(e).includes('no active session')) notify('err', String(e))
    } finally {
      setBusy(false)
    }
  }

  const patch = useCallback((id: number, p: Partial<Screenshot>) => {
    setShots((prev) => prev.map((s) => (s.id === id ? { ...s, ...p } : s)))
  }, [])

  const remove = async (id: number) => {
    try {
      await api.deleteScreenshot(id)
      setShots((prev) => prev.filter((s) => s.id !== id))
    } catch (e) {
      notify('err', String(e))
    }
  }

  const analyze = async (id: number, notes: string) => {
    patch(id, { analyzing: true })
    try {
      await api.analyze(id, notes, globalPrompt)
    } catch (e) {
      patch(id, { analyzing: false })
      notify('err', String(e))
    }
  }

  const analyzeAll = (onlyMissing: boolean) =>
    shots
      .filter((s) => !s.analyzing && (!onlyMissing || !s.aiSummary || !!s.aiError))
      .forEach((s) => analyze(s.id, s.userNotes))

  const exportDoc = async () => {
    try {
      const folder = await api.chooseExportFolder()
      if (!folder) return
      const items: FrontendScreenshotData[] = shots.map((s) => ({ id: s.id, userNotes: s.userNotes, aiSummary: s.aiSummary }))
      await api.exportMarkdown(sessionId, items, folder)
      notify('ok', `Exported ${items.length} steps to ${folder}`)
    } catch (e) {
      notify('err', String(e))
    }
  }

  const analyzingCount = useMemo(() => shots.filter((s) => s.analyzing).length, [shots])

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-40 border-b border-slate-800 bg-slate-950/90 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
          <h1 className="text-lg font-semibold tracking-tight text-white">
            📸 Screenshotinator <span className="ml-2 text-sm font-normal text-slate-500">session recall</span>
          </h1>
          <div className="flex items-center gap-3">
          <button onClick={() => setShowSettings(true)} className="rounded-lg border border-slate-700 px-3 py-2 text-sm text-slate-300 hover:bg-slate-800" title="LLM settings">
            ⚙ Settings
          </button>
          {phase === 'review' && (
            <>
              {analyzingCount > 0 && <span className="text-xs text-slate-400">{analyzingCount} analyzing…</span>}
              <button onClick={() => setPhase('idle')} className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800">
                Discard view
              </button>
              <button
                onClick={exportDoc}
                disabled={shots.length === 0}
                className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
              >
                ⬇ Export Markdown
              </button>
            </>
          )}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-6 py-8">
        {phase === 'idle' && (
          <div className="mx-auto flex max-w-xl flex-col items-center gap-6 py-24 text-center">
            <h2 className="text-3xl font-semibold text-white">Record a work session</h2>
            <p className="text-slate-400">
              Press start — this window hides, and every click captures the focused window. Press{' '}
              <kbd className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">{hotkey}</kbd> to stop recording and open the review. Nothing is sent to the AI until you start it.
            </p>
            <button
              onClick={start}
              className="rounded-xl bg-indigo-600 px-8 py-3 font-semibold text-white shadow-lg shadow-indigo-900/40 transition hover:bg-indigo-500"
            >
              ● Start Session
            </button>
          </div>
        )}

        {phase === 'recording' && <RecordingPanel hotkey={hotkey} count={recCount} busy={busy} onEnd={end} />}

        {phase === 'review' &&
          (shots.length === 0 ? (
            <p className="py-24 text-center text-slate-500">No screenshots left in this session.</p>
          ) : (
            <>
              <p className="mb-4 text-sm text-slate-400">
                {shots.length} step{shots.length === 1 ? '' : 's'} — remove what you don't need, add notes, run the AI, then export.
              </p>
              <div className="mb-6 rounded-2xl border border-slate-800 bg-slate-900 p-4">
                <label className="mb-1 block text-xs font-medium uppercase tracking-wide text-slate-500">
                  Instructions for all steps <span className="normal-case text-slate-600">– sent with every analysis, plus each step's own notes</span>
                </label>
                <textarea
                  value={globalPrompt}
                  onChange={(e) => setGlobalPrompt(e.target.value)}
                  rows={2}
                  placeholder="e.g. “This is a Kubernetes debugging session. Name pods, namespaces and error codes you can see.”"
                  className="w-full resize-y rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 placeholder-slate-600 outline-none focus:border-indigo-500"
                />
                <div className="mt-3 flex flex-wrap gap-3">
                  <button onClick={() => analyzeAll(true)} className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500">
                    ✨ Analyze all unanalyzed
                  </button>
                  <button onClick={() => analyzeAll(false)} className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800">
                    ↻ Re-analyze everything
                  </button>
                </div>
              </div>
              <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
                {shots.map((s, i) => (
                  <ScreenshotCard
                    key={s.id}
                    index={i}
                    shot={s}
                    onNotesChange={(n) => patch(s.id, { userNotes: n })}
                    onNotesCommit={(n) => api.updateNotes(s.id, n).catch((e) => notify('err', String(e)))}
                    onDelete={() => remove(s.id)}
                    onAnalyze={() => analyze(s.id, s.userNotes)}
                  />
                ))}
              </div>
            </>
          ))}
      </main>

      {showSettings && (
        <SettingsDialog
          onClose={() => setShowSettings(false)}
          onSaved={() => notify('ok', 'LLM settings saved')}
          onError={(m) => notify('err', m)}
        />
      )}

      {toast && (
        <div
          className={`fixed bottom-6 right-6 z-50 max-w-md rounded-xl px-4 py-3 text-sm shadow-xl ${
            toast.kind === 'ok' ? 'bg-emerald-700 text-white' : 'bg-rose-700 text-white'
          }`}
        >
          {toast.text}
        </div>
      )}
    </div>
  )
}
