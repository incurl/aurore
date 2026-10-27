import { describe, expect, it } from 'vitest';

import { formatDetail } from './ConsolePanel';
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
