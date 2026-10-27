import { afterEach, describe, expect, it } from 'vitest';

import { applyZoom, clampZoom, MAX_ZOOM, MIN_ZOOM, ZOOM_PRESETS } from './zoom';

describe('clampZoom', () => {
  it('keeps a factor inside the supported range', () => {
    expect(clampZoom(100)).toBe(100);
    expect(clampZoom(50)).toBe(50);
    expect(clampZoom(200)).toBe(200);
  });

  it('clamps a factor outside the range', () => {
    expect(clampZoom(10)).toBe(MIN_ZOOM);
    expect(clampZoom(500)).toBe(MAX_ZOOM);
    expect(clampZoom(-20)).toBe(MIN_ZOOM);
  });

  it('rounds to a whole percentage', () => {
    expect(clampZoom(123.4)).toBe(123);
    expect(clampZoom(123.6)).toBe(124);
  });

  it('falls back to 100 for a non-finite factor', () => {
    // Neither NaN nor an infinity is a usable factor, so both fall
    // back to the default rather than to a limit.
    expect(clampZoom(Number.NaN)).toBe(100);
    expect(clampZoom(Number.POSITIVE_INFINITY)).toBe(100);
    expect(clampZoom(Number.NEGATIVE_INFINITY)).toBe(100);
  });
});

describe('applyZoom', () => {
  afterEach(() => {
    document.documentElement.removeAttribute('style');
    delete document.documentElement.dataset.zoom;
  });

  it('scales the root font size', () => {
    applyZoom(150);
    expect(document.documentElement.style.fontSize).toBe('19.5px');

    applyZoom(100);
    expect(document.documentElement.style.fontSize).toBe('13px');

    applyZoom(50);
    expect(document.documentElement.style.fontSize).toBe('6.5px');
  });

  it('exposes the factor so the stylesheet can use it', () => {
    applyZoom(125);
    expect(document.documentElement.style.getPropertyValue('--zoom')).toBe('1.25');
    expect(document.documentElement.dataset.zoom).toBe('125');
  });

  it('clamps an out-of-range factor', () => {
    applyZoom(1000);
    expect(document.documentElement.style.fontSize).toBe('26px');
  });
});

describe('the zoom presets', () => {
  it('are all inside the supported range', () => {
    for (const preset of ZOOM_PRESETS) {
      expect(preset).toBeGreaterThanOrEqual(MIN_ZOOM);
      expect(preset).toBeLessThanOrEqual(MAX_ZOOM);
    }
  });

  it('include 100, the default', () => {
    expect(ZOOM_PRESETS).toContain(100);
  });
});
