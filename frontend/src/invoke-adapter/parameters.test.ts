import { describe, expect, it } from 'vitest';

import {
  applyCapabilities,
  clampBatchSize,
  clampDimension,
  clampPrompt,
  DEFAULT_FORM_STATE,
  describeErrorCode,
  isValidDimensionPair,
  MAX_PROMPT_LENGTH,
  supportsDimensions,
  supportsStyle,
  toGenerationRequest,
  validateForm,
} from './parameters';
import type { Capabilities, GenerationFormState } from './types';

const baseForm = (overrides: Partial<GenerationFormState> = {}): GenerationFormState => ({
  ...DEFAULT_FORM_STATE,
  prompt: 'a red bicycle',
  ...overrides,
});

const capabilities = (overrides: Partial<Capabilities> = {}): Capabilities => ({
  model: 'image-01',
  supports_dimensions: true,
  supports_style: false,
  supports_reference_image: true,
  supports_negative_prompt: false,
  supports_mask: false,
  aspect_ratios: ['1:1', '16:9', '4:3', '3:2', '2:3', '3:4', '9:16', '21:9'],
  style_types: ['漫画', '元气', '中世纪', '水彩'],
  min_dimension: 512,
  max_dimension: 2048,
  max_batch_size: 9,
  max_prompt_length: 1500,
  ...overrides,
});

describe('clampBatchSize', () => {
  it('keeps valid batch sizes', () => {
    expect(clampBatchSize(1)).toBe(1);
    expect(clampBatchSize(5)).toBe(5);
    expect(clampBatchSize(9)).toBe(9);
  });

  it('clamps out-of-range batch sizes', () => {
    expect(clampBatchSize(0)).toBe(1);
    expect(clampBatchSize(-4)).toBe(1);
    expect(clampBatchSize(10)).toBe(9);
    expect(clampBatchSize(100)).toBe(9);
  });

  it('handles non-finite input', () => {
    expect(clampBatchSize(Number.NaN)).toBe(1);
  });
});

describe('clampDimension', () => {
  it('snaps to a multiple of 8', () => {
    expect(clampDimension(1023)).toBe(1024);
    expect(clampDimension(1000)).toBe(1000);
  });

  it('clamps into the allowed range', () => {
    expect(clampDimension(10)).toBe(512);
    expect(clampDimension(99999)).toBe(2048);
  });
});

describe('isValidDimensionPair', () => {
  it('accepts a valid pair', () => {
    expect(isValidDimensionPair(1024, 768)).toBe(true);
  });

  it('rejects out-of-range values', () => {
    expect(isValidDimensionPair(500, 500)).toBe(false);
    expect(isValidDimensionPair(3000, 1000)).toBe(false);
  });

  it('rejects values that are not multiples of 8', () => {
    expect(isValidDimensionPair(1023, 768)).toBe(false);
  });

  it('rejects incomplete pairs', () => {
    expect(isValidDimensionPair(1024, 0)).toBe(false);
  });
});

describe('clampPrompt', () => {
  it('truncates long prompts', () => {
    const long = 'x'.repeat(MAX_PROMPT_LENGTH + 100);
    expect(clampPrompt(long)).toHaveLength(MAX_PROMPT_LENGTH);
  });

  it('keeps short prompts unchanged', () => {
    expect(clampPrompt('hello')).toBe('hello');
  });
});

describe('model capability helpers', () => {
  it('only image-01 supports dimensions', () => {
    expect(supportsDimensions('image-01')).toBe(true);
    expect(supportsDimensions('image-01-live')).toBe(false);
  });

  it('only image-01-live supports style', () => {
    expect(supportsStyle('image-01')).toBe(false);
    expect(supportsStyle('image-01-live')).toBe(true);
  });
});

describe('toGenerationRequest', () => {
  it('maps the core fields', () => {
    const request = toGenerationRequest(
      baseForm({ aspectRatio: '16:9', width: 1280, height: 720, batchSize: 3, seed: 42 }),
    );

    expect(request.prompt).toBe('a red bicycle');
    expect(request.params.aspect_ratio).toBe('16:9');
    expect(request.params.width).toBe(1280);
    expect(request.params.height).toBe(720);
    expect(request.params.batch_size).toBe(3);
    expect(request.params.seed).toBe(42);
  });

  it('never emits unsupported diffusion controls', () => {
    const request = toGenerationRequest(baseForm());
    const serialised = JSON.stringify(request);

    for (const forbidden of [
      'cfg_scale',
      'cfg',
      'sampler',
      'steps',
      'scheduler',
      'lora',
      'controlnet',
      'denoising_strength',
    ]) {
      expect(serialised).not.toContain(forbidden);
    }
  });

  it('drops the aspect ratio and dimensions for image-01-live', () => {
    const request = toGenerationRequest(
      baseForm({ model: 'image-01-live', aspectRatio: '16:9', width: 1280, height: 720 }),
    );

    expect(request.params.aspect_ratio).toBe('');
    expect(request.params.width).toBe(0);
    expect(request.params.height).toBe(0);
  });

  it('drops the style preset for image-01', () => {
    const request = toGenerationRequest(
      baseForm({ model: 'image-01', styleType: '水彩', styleWeight: 0.5 }),
    );

    expect(request.params.style_type).toBe('');
    expect(request.params.style_weight).toBe(0);
  });

  it('keeps the style preset for image-01-live', () => {
    const request = toGenerationRequest(
      baseForm({ model: 'image-01-live', styleType: '水彩', styleWeight: 0.5 }),
    );

    expect(request.params.style_type).toBe('水彩');
    expect(request.params.style_weight).toBe(0.5);
  });

  it('clamps the batch size', () => {
    expect(toGenerationRequest(baseForm({ batchSize: 50 })).params.batch_size).toBe(9);
    expect(toGenerationRequest(baseForm({ batchSize: 0 })).params.batch_size).toBe(1);
  });

  it('clamps and snaps the dimensions', () => {
    const request = toGenerationRequest(baseForm({ width: 1023, height: 999 }));
    expect(request.params.width % 8).toBe(0);
    expect(request.params.height % 8).toBe(0);
  });

  it('trims the prompt and the negative prompt', () => {
    const request = toGenerationRequest(baseForm({ prompt: '  hello  ', negativePrompt: '  ugly ' }));
    expect(request.prompt).toBe('hello');
    expect(request.negative_prompt).toBe('ugly');
  });

  it('carries the negative prompt toggle through', () => {
    const request = toGenerationRequest(
      baseForm({ negativePrompt: 'blurry', useNegativePrompt: true }),
    );
    expect(request.params.use_negative_prompt).toBe(true);
    expect(request.negative_prompt).toBe('blurry');
  });
});

describe('validateForm', () => {
  it('accepts a valid form', () => {
    expect(validateForm(baseForm())).toEqual([]);
  });

  it('requires a prompt', () => {
    const problems = validateForm(baseForm({ prompt: '   ' }));
    expect(problems.length).toBeGreaterThan(0);
    expect(problems[0]).toMatch(/prompt/i);
  });

  it('rejects an over-long prompt', () => {
    const problems = validateForm(baseForm({ prompt: 'x'.repeat(MAX_PROMPT_LENGTH + 1) }));
    expect(problems.some((p) => p.includes('at most'))).toBe(true);
  });

  it('rejects invalid dimensions', () => {
    const problems = validateForm(baseForm({ width: 100, height: 100 }));
    expect(problems.some((p) => p.includes('multiple of 8'))).toBe(true);
  });

  it('does not validate dimensions for image-01-live', () => {
    const problems = validateForm(
      baseForm({ model: 'image-01-live', width: 100, height: 100 }),
    );
    expect(problems.some((p) => p.includes('multiple of 8'))).toBe(false);
  });

  it('rejects an out-of-range batch size', () => {
    expect(validateForm(baseForm({ batchSize: 12 })).length).toBeGreaterThan(0);
  });

  it('rejects an invalid style weight', () => {
    const problems = validateForm(
      baseForm({ model: 'image-01-live', styleType: '水彩', styleWeight: 1.5 }),
    );
    expect(problems.some((p) => p.includes('style weight'))).toBe(true);
  });
});

describe('applyCapabilities', () => {
  it('clears the dimensions when the model does not support them', () => {
    const form = applyCapabilities(
      baseForm({ width: 1280, height: 720 }),
      capabilities({ supports_dimensions: false }),
    );

    expect(form.width).toBe(0);
    expect(form.height).toBe(0);
  });

  it('clears the style when the model does not support it', () => {
    const form = applyCapabilities(
      baseForm({ styleType: '水彩', styleWeight: 0.5 }),
      capabilities({ supports_style: false }),
    );

    expect(form.styleType).toBe('');
    expect(form.styleWeight).toBe(0);
  });

  it('falls back to a valid aspect ratio', () => {
    const form = applyCapabilities(
      baseForm({ aspectRatio: '4:3' }),
      capabilities({ aspect_ratios: ['1:1', '16:9'] }),
    );

    expect(form.aspectRatio).toBe('1:1');
  });
});

describe('describeErrorCode', () => {
  it('describes the documented MiniMax status codes', () => {
    expect(describeErrorCode(1002)).toMatch(/rate limited/i);
    expect(describeErrorCode(1004)).toMatch(/api key/i);
    expect(describeErrorCode(1008)).toMatch(/balance/i);
    expect(describeErrorCode(1026)).toMatch(/sensitive/i);
    expect(describeErrorCode(2013)).toMatch(/parameters/i);
    expect(describeErrorCode(2049)).toMatch(/invalid/i);
  });

  it('returns an empty string for an unknown code', () => {
    expect(describeErrorCode(0)).toBe('');
  });

  it('describes an unreadable response', () => {
    expect(describeErrorCode(-1)).toMatch(/could not read/i);
    expect(describeErrorCode(-1)).toMatch(/debug console/i);
  });
});
