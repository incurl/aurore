import { describe, expect, it } from 'vitest';

import { canDelete } from './JobQueue';

describe('canDelete', () => {
  it('allows a finished job to be removed', () => {
    expect(canDelete('completed')).toBe(true);
    expect(canDelete('failed')).toBe(true);
    expect(canDelete('cancelled')).toBe(true);
  });

  it('refuses a job that has not come to rest', () => {
    expect(canDelete('pending')).toBe(false);
    expect(canDelete('running')).toBe(false);
  });
});
