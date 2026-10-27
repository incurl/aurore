import { describe, expect, it, vi, afterEach } from 'vitest';

import {
  entriesToText,
  entryToText,
  formatDetail,
  hasTextSelection,
  writeClipboard,
} from './ConsolePanel';
import type { ConsoleEntry } from '../../wails-bindings';

/**
 * Builds a console entry for the tests.
 */
function entry(overrides: Partial<ConsoleEntry>): ConsoleEntry {
  return {
    id: 1,
    time: '12:00:00.000',
    level: 'info',
    source: 'job abc',
    message: 'hello',
    detail: '',
    ...overrides,
  };
}

describe('formatDetail', () => {
  it('pretty prints a JSON body', () => {
    const got = formatDetail('{"base_resp":{"status_code":2013}}');
    expect(got).toContain('"status_code": 2013');
    expect(got.split('\n').length).toBeGreaterThan(1);
  });

  it('shows a non-JSON body verbatim', () => {
    expect(formatDetail('not json at all')).toBe('not json at all');
  });

  it('returns an empty string for a blank detail', () => {
    expect(formatDetail('')).toBe('');
    expect(formatDetail('   \n  ')).toBe('');
  });

  it('keeps a JSON body readable when it is an array', () => {
    const got = formatDetail('[1,2,3]');
    expect(got).toBe('[\n  1,\n  2,\n  3\n]');
  });
});

describe('the console entries', () => {
  it('carry the fields the panel renders', () => {
    const e = entry({ level: 'raw', message: 'MiniMax error 2013' });
    expect(e.time).toBe('12:00:00.000');
    expect(e.level).toBe('raw');
    expect(e.source).toBe('job abc');
    expect(e.message).toBe('MiniMax error 2013');
  });
});

describe('entryToText', () => {
  it('renders the header line', () => {
    const got = entryToText(entry({}));
    expect(got).toBe('12:00:00.000 [info] job abc: hello');
  });

  it('appends the pretty printed detail', () => {
    const got = entryToText(entry({ detail: '{"a":1}' }));
    expect(got).toContain('12:00:00.000 [info] job abc: hello');
    expect(got).toContain('"a": 1');
  });
});

describe('entriesToText', () => {
  it('joins the entries one per block', () => {
    const got = entriesToText([
      entry({ id: 1, message: 'first' }),
      entry({ id: 2, message: 'second' }),
    ]);
    const lines = got.split('\n');
    expect(lines).toHaveLength(2);
    expect(lines[0]).toContain('first');
    expect(lines[1]).toContain('second');
  });

  it('returns an empty string for no entries', () => {
    expect(entriesToText([])).toBe('');
  });
});

describe('hasTextSelection', () => {
  afterEach(() => {
    window.getSelection()?.removeAllRanges();
  });

  it('is false with nothing selected', () => {
    expect(hasTextSelection()).toBe(false);
  });

  it('is true when text is selected', () => {
    const el = document.createElement('div');
    el.textContent = 'selectable text';
    document.body.appendChild(el);

    const range = document.createRange();
    range.selectNodeContents(el);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);

    expect(hasTextSelection()).toBe(true);
    document.body.removeChild(el);
  });

  it('ignores a whitespace-only selection', () => {
    const el = document.createElement('div');
    el.textContent = '   ';
    document.body.appendChild(el);

    const range = document.createRange();
    range.selectNodeContents(el);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);

    expect(hasTextSelection()).toBe(false);
    document.body.removeChild(el);
  });
});

describe('writeClipboard', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('uses the async clipboard when it is available', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });

    await expect(writeClipboard('hello')).resolves.toBe(true);
    expect(writeText).toHaveBeenCalledWith('hello');
  });

  it('falls back to the copy command when the async clipboard fails', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('not allowed'));
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    const execCommand = vi.fn().mockReturnValue(true);
    document.execCommand = execCommand;

    await expect(writeClipboard('hello')).resolves.toBe(true);
    expect(execCommand).toHaveBeenCalledWith('copy');
  });

  it('falls back when the async clipboard is missing', async () => {
    vi.stubGlobal('navigator', {});
    const execCommand = vi.fn().mockReturnValue(true);
    document.execCommand = execCommand;

    await expect(writeClipboard('hello')).resolves.toBe(true);
    expect(execCommand).toHaveBeenCalledWith('copy');
  });

  it('reports failure when both paths fail', async () => {
    vi.stubGlobal('navigator', {});
    document.execCommand = vi.fn().mockReturnValue(false);

    await expect(writeClipboard('hello')).resolves.toBe(false);
  });
});
