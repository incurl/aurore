/**
 * The debug console.
 *
 * A log view at the bottom of the window that records what the
 * application and the MiniMax API did: the requests that were sent, the
 * responses that came back and the errors that were raised.
 *
 * It is deliberately a plain text view rather than a rich editor: the
 * point is to read a raw API response verbatim.
 */

import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import type { ConsoleEntry } from '../../wails-bindings';

interface ConsolePanelProps {
  /** The recorded entries, oldest first. */
  entries: ConsoleEntry[];
  /** Discards the recorded entries. */
  onClear: () => void;
}

type LevelFilter = 'all' | 'error' | 'raw';

/** The levels that the filter can show. */
const LEVEL_FILTERS: { value: LevelFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'error', label: 'Errors' },
  { value: 'raw', label: 'Raw' },
];

/**
 * Decides whether an entry passes the level filter.
 */
function matchesFilter(entry: ConsoleEntry, filter: LevelFilter): boolean {
  switch (filter) {
    case 'error':
      return entry.level === 'error' || entry.level === 'warn';
    case 'raw':
      return entry.level === 'raw';
    default:
      return true;
  }
}

/**
 * Formats the entry detail for display.
 *
 * A raw API body is pretty printed when it is valid JSON, so that it can
 * be read, and shown verbatim otherwise.
 */
export function formatDetail(detail: string): string {
  const trimmed = detail.trim();
  if (!trimmed) {
    return '';
  }
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return detail;
  }
}

/**
 * The debug console panel.
 */
export function ConsolePanel({ entries, onClear }: ConsolePanelProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [filter, setFilter] = useState<LevelFilter>('all');
  const [follow, setFollow] = useState(true);
  const [selected, setSelected] = useState<number | null>(null);
  const [copied, setCopied] = useState(false);
  const bodyRef = useRef<HTMLDivElement | null>(null);

  const visible = entries.filter((entry) => matchesFilter(entry, filter));
  const errorCount = entries.filter(
    (entry) => entry.level === 'error' || entry.level === 'warn',
  ).length;

  // Scroll to the newest entry, unless the user scrolled up to read
  // something.
  useLayoutEffect(() => {
    if (!follow || collapsed) {
      return;
    }
    const body = bodyRef.current;
    if (body) {
      body.scrollTop = body.scrollHeight;
    }
  }, [visible.length, follow, collapsed]);

  // Clear the copied marker after a moment.
  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copyAll = async () => {
    const text = visible
      .map((entry) => {
        const detail = formatDetail(entry.detail);
        const head = `${entry.time} [${entry.level}] ${entry.source}: ${entry.message}`;
        return detail ? `${head}\n${detail}` : head;
      })
      .join('\n');
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      // The clipboard is unavailable in some webview configurations, so
      // the failure is ignored: the text is still on screen.
    }
  };

  const active =
    selected === null ? null : entries.find((entry) => entry.id === selected) ?? null;

  return (
    <section className={`console-panel${collapsed ? ' console-panel-collapsed' : ''}`}>
      <div className="console-header">
        <button
          type="button"
          className="console-toggle"
          onClick={() => setCollapsed((value) => !value)}
          aria-expanded={!collapsed}
        >
          <span className="console-caret">{collapsed ? '▸' : '▾'}</span>
          Console
        </button>

        <span className="text-dim">
          {visible.length} entr{visible.length === 1 ? 'y' : 'ies'}
          {errorCount > 0 && (
            <span className="console-error-count">
              {' · '}
              {errorCount} error{errorCount === 1 ? '' : 's'}
            </span>
          )}
        </span>

        <span className="grow" />

        {LEVEL_FILTERS.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`btn btn-sm${filter === option.value ? ' btn-active' : ''}`}
            onClick={() => setFilter(option.value)}
          >
            {option.label}
          </button>
        ))}

        <button
          type="button"
          className={`btn btn-sm${follow ? ' btn-active' : ''}`}
          onClick={() => setFollow((value) => !value)}
          title="Scroll to the newest entry"
        >
          Follow
        </button>

        <button type="button" className="btn btn-sm" onClick={() => void copyAll()}>
          {copied ? 'Copied' : 'Copy'}
        </button>

        <button
          type="button"
          className="btn btn-sm"
          onClick={() => {
            setSelected(null);
            onClear();
          }}
        >
          Clear
        </button>
      </div>

      {!collapsed && (
        <div className="console-body" ref={bodyRef}>
          {visible.length === 0 ? (
            <div className="console-empty">
              No console output yet. API requests and errors appear here.
            </div>
          ) : (
            visible.map((entry) => (
              <div
                key={entry.id}
                className={`console-entry console-level-${entry.level}${
                  selected === entry.id ? ' console-entry-selected' : ''
                }`}
                onClick={() => setSelected(selected === entry.id ? null : entry.id)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    setSelected(selected === entry.id ? null : entry.id);
                  }
                }}
              >
                <span className="console-time">{entry.time}</span>
                <span className="console-level">{entry.level}</span>
                <span className="console-source">{entry.source}</span>
                <span className="console-message">{entry.message}</span>
              </div>
            ))
          )}
        </div>
      )}

      {!collapsed && active?.detail && (
        <div className="console-detail">
          <div className="console-detail-head">
            <span>
              {active.time} · {active.source}
            </span>
            <span className="grow" />
            <button type="button" className="btn btn-sm" onClick={() => void copyAll()}>
              {copied ? 'Copied' : 'Copy all'}
            </button>
          </div>
          <pre className="console-detail-body">{formatDetail(active.detail)}</pre>
        </div>
      )}
    </section>
  );
}
