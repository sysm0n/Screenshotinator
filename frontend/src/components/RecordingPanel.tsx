interface Props {
  hotkey: string
  count: number
  busy: boolean
  onEnd: () => void
}

export default function RecordingPanel({ hotkey, count, busy, onEnd }: Props) {
  return (
    <div className="mx-auto flex max-w-lg flex-col items-center gap-6 py-24 text-center">
      <span className="relative flex h-5 w-5">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-rose-500 opacity-75" />
        <span className="relative inline-flex h-5 w-5 rounded-full bg-rose-500" />
      </span>
      <h2 className="text-2xl font-semibold text-white">Session is recording</h2>
      <p className="text-slate-400">
        Every click captures the focused window (max one per 1.5 s). <strong className="text-slate-200">{count}</strong>{' '}
        step{count === 1 ? '' : 's'} so far.
      </p>
      <p className="text-sm text-slate-500">
        Press <kbd className="rounded bg-slate-800 px-1.5 py-0.5 text-slate-200">{hotkey}</kbd> any time to stop and review.
      </p>
      <button
        onClick={onEnd}
        disabled={busy}
        className="rounded-xl bg-rose-600 px-8 py-3 font-semibold text-white shadow-lg shadow-rose-900/40 transition hover:bg-rose-500 disabled:opacity-60"
      >
        {busy ? 'Finishing…' : '■ End Session'}
      </button>
    </div>
  )
}
