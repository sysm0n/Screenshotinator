import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { Screenshot } from '../lib/types'

interface Props {
  index: number
  shot: Screenshot
  onNotesChange: (notes: string) => void
  onNotesCommit: (notes: string) => void
  onDelete: () => void
  onAnalyze: () => void
}

export default function ScreenshotCard({ index, shot, onNotesChange, onNotesCommit, onDelete, onAnalyze }: Props) {
  const [zoom, setZoom] = useState(false)
  const [src, setSrc] = useState('')

  useEffect(() => {
    let alive = true
    api.getImage(shot.id).then((d) => alive && setSrc(d)).catch(() => {})
    return () => {
      alive = false
    }
  }, [shot.id])

  return (
    <article className="flex flex-col overflow-hidden rounded-2xl border border-slate-800 bg-slate-900 shadow-lg shadow-black/20">
      <button
        onClick={() => setZoom(true)}
        className="group relative block aspect-video w-full overflow-hidden bg-slate-950"
        title="Click to enlarge"
      >
        <img src={src} alt={`Step ${index + 1}`} className="h-full w-full object-contain transition group-hover:scale-[1.02]" />
        <span className="absolute left-3 top-3 rounded-full bg-indigo-600 px-2.5 py-0.5 text-xs font-semibold text-white">
          Step {index + 1}
        </span>
      </button>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <header>
          <h3 className="truncate text-sm font-semibold text-slate-100" title={shot.appName}>
            {shot.appName || 'Unknown window'}
          </h3>
          <time className="text-xs text-slate-500">{new Date(shot.timestamp).toLocaleString()}</time>
        </header>

        <section>
          <div className="mb-1 flex items-center justify-between">
            <span className="text-xs font-medium uppercase tracking-wide text-slate-500">AI summary</span>
            <button
              onClick={onAnalyze}
              disabled={shot.analyzing}
              className="rounded-md border border-indigo-500/40 px-2 py-0.5 text-xs text-indigo-300 transition hover:bg-indigo-500/10 disabled:opacity-50"
            >
              {shot.analyzing ? 'Analyzing…' : shot.aiSummary ? '↻ AI Re-Analyze' : '✨ AI Analyze'}
            </button>
          </div>
          <p className="min-h-[3.5rem] whitespace-pre-wrap text-sm leading-relaxed text-slate-300">
            {shot.analyzing && !shot.aiSummary ? (
              <span className="animate-pulse text-slate-500">Waiting for the model…</span>
            ) : (
              shot.aiSummary || <span className="text-slate-600">Not analyzed yet.</span>
            )}
          </p>
          {shot.aiError && !shot.analyzing && (
            <p className="mt-2 rounded-md bg-amber-500/10 px-2 py-1 text-xs text-amber-400">⚠ AI analysis failed: {shot.aiError}</p>
          )}
        </section>

        <section>
          <label className="mb-1 block text-xs font-medium uppercase tracking-wide text-slate-500">Your notes <span className="normal-case text-slate-600">– used as instructions when you run AI Analyze</span></label>
          <textarea
            value={shot.userNotes}
            onChange={(e) => onNotesChange(e.target.value)}
            onBlur={(e) => onNotesCommit(e.target.value)}
            rows={3}
            placeholder="Describe this step, or instruct the AI (e.g. “focus on the error message”) and run AI Analyze…"
            className="w-full resize-y rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 placeholder-slate-600 outline-none focus:border-indigo-500"
          />
        </section>

        <button
          onClick={onDelete}
          className="mt-auto self-end rounded-md border border-rose-500/40 px-3 py-1 text-xs text-rose-300 transition hover:bg-rose-500/10"
        >
          🗑 Remove
        </button>
      </div>

      {zoom && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-6" onClick={() => setZoom(false)}>
          <img src={src} alt="" className="max-h-full max-w-full rounded-lg shadow-2xl" />
        </div>
      )}
    </article>
  )
}
