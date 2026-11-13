/**
 * The Invoke adapter.
 *
 * This module is the only place where the Invoke-style UI payload is
 * translated into the request that the Go backend understands. It is a
 * pure function: no side effects, no I/O, no business logic beyond
 * shape and validation.
 *
 * The Go backend remains the authority on what actually reaches MiniMax
 * (see `minimax.BuildPayload`), but the adapter rejects obviously invalid
 * input early so the user gets immediate feedback instead of a failed
 * job.
 */

import type {
  AspectRatio,
  Capabilities,
  GenerationFormState,
  GenerationRequest,
  ImageArea,
  MiniMaxModel,
  StyleType,
} from './types';

/** The default form state, matching the Go defaults. */
export const DEFAULT_FORM_STATE: GenerationFormState = {
  prompt: '',
  negativePrompt: '',
  model: 'image-01',
  aspectRatio: '1:1',
  width: 1024,
  height: 1024,
  area: '2K',
  batchSize: 1,
  seed: 0,
  promptOptimizer: false,
  watermark: false,
  styleType: '',
  styleWeight: 0.8,
  useNegativePrompt: false,
  maskMode: 'generated_in_mask',
  referenceAssetId: '',
  maskAssetId: '',
};

/** The minimum dimension accepted by MiniMax. */
export const MIN_DIMENSION = 512;

/** The maximum dimension accepted by MiniMax. */
export const MAX_DIMENSION = 2048;

/** The dimension multiple required by MiniMax. */
export const DIMENSION_MULTIPLE = 8;

/** The largest batch MiniMax accepts. */
export const MAX_BATCH_SIZE = 9;

/** The largest batch the H3 models accept. */
export const H3_MAX_BATCH_SIZE = 10;

/** The H3 output area levels. */
export const IMAGE_AREAS: ImageArea[] = ['1K', '2K', '4K'];

/** The prompt character limit. */
export const MAX_PROMPT_LENGTH = 1500;

/** The prompt character limit for the H3 models. */
export const H3_MAX_PROMPT_LENGTH = 32000;

/** The aspect ratios MiniMax accepts. */
export const ASPECT_RATIOS: AspectRatio[] = [
  '1:1',
  '16:9',
  '4:3',
  '3:2',
  '2:3',
  '3:4',
  '9:16',
  '21:9',
];

/** The style presets MiniMax accepts. */
export const STYLE_TYPES: StyleType[] = ['漫画', '元气', '中世纪', '水彩'];

/** The models MiniMax offers. */
export const MODELS: MiniMaxModel[] = [
  'image-01',
  'image-01-live',
  'MiniMax-H3-Image',
  'MiniMax-H3-Image-Highspeed',
];

/**
 * Reports whether the model belongs to the H3 family.
 *
 * The H3 models are a different API from Image-01: they have no seed,
 * style or watermark, and they size from a ratio and an area.
 *
 * It takes a plain string because the selected model comes back from the
 * backend as one, and a value this does not recognise is simply not H3.
 */
export function isH3Model(model: string): boolean {
  return model === 'MiniMax-H3-Image' || model === 'MiniMax-H3-Image-Highspeed';
}

/**
 * Reports whether a width/height pair satisfies the MiniMax constraints:
 * both set, within [512, 2048] and each a multiple of 8.
 */
export function isValidDimensionPair(width: number, height: number): boolean {
  return (
    Number.isInteger(width) &&
    Number.isInteger(height) &&
    width >= MIN_DIMENSION &&
    width <= MAX_DIMENSION &&
    height >= MIN_DIMENSION &&
    height <= MAX_DIMENSION &&
    width % DIMENSION_MULTIPLE === 0 &&
    height % DIMENSION_MULTIPLE === 0
  );
}

/**
 * Clamps a batch size into the range MiniMax accepts.
 */
export function clampBatchSize(batchSize: number, max = MAX_BATCH_SIZE): number {
  if (!Number.isFinite(batchSize)) {
    return 1;
  }
  return Math.min(max, Math.max(1, Math.trunc(batchSize)));
}

/**
 * Clamps a dimension into the range MiniMax accepts, snapped to the
 * required multiple of 8.
 */
export function clampDimension(value: number): number {
  if (!Number.isFinite(value)) {
    return MIN_DIMENSION;
  }
  const snapped = Math.round(value / DIMENSION_MULTIPLE) * DIMENSION_MULTIPLE;
  return Math.min(MAX_DIMENSION, Math.max(MIN_DIMENSION, snapped));
}

/**
 * Truncates a prompt to the MiniMax character limit.
 */
export function clampPrompt(prompt: string, limit = MAX_PROMPT_LENGTH): string {
  return prompt.length > limit ? prompt.slice(0, limit) : prompt;
}

/**
 * Reports whether the model accepts explicit dimensions.
 *
 * Only `image-01` accepts a raw width and height. The H3 models size
 * their output from an aspect ratio and an area level instead.
 */
export function supportsDimensions(model: string): boolean {
  return model === 'image-01';
}

/**
 * Reports whether the model accepts an aspect ratio.
 */
export function supportsAspectRatio(model: string): boolean {
  return model !== 'image-01-live';
}

/**
 * Reports whether the model accepts a style preset.
 *
 * Only `image-01-live` supports the style object.
 */
export function supportsStyle(model: string): boolean {
  return model === 'image-01-live';
}

/**
 * Converts the UI form state into the backend request.
 *
 * The conversion applies the same rules as the Go side so that the
 * request the backend receives is already normalised:
 *
 * - the prompt is truncated to the character limit;
 * - the batch size is clamped to [1, 9];
 * - the dimensions are clamped and snapped to a multiple of 8;
 * - the aspect ratio is dropped for `image-01-live`;
 * - the style preset is dropped for `image-01`;
 * - the seed is passed through untouched (0 means "server picks").
 */
export function toGenerationRequest(form: GenerationFormState): GenerationRequest {
  const h3 = isH3Model(form.model);
  const explicitSize = supportsDimensions(form.model);
  const maxBatch = h3 ? H3_MAX_BATCH_SIZE : MAX_BATCH_SIZE;

  const params = {
    model: form.model,
    aspect_ratio: supportsAspectRatio(form.model) ? form.aspectRatio : '',
    width: explicitSize ? clampDimension(form.width) : 0,
    height: explicitSize ? clampDimension(form.height) : 0,
    // The area level is only meaningful to H3, and is dropped for the
    // Image-01 family so it cannot leak into a request that has no such
    // field.
    area: h3 ? form.area : '',
    batch_size: clampBatchSize(form.batchSize, maxBatch),
    seed: h3 ? 0 : form.seed,
    prompt_optimizer: form.promptOptimizer,
    watermark: h3 ? false : form.watermark,
    style_type: supportsStyle(form.model) ? form.styleType : '',
    style_weight: supportsStyle(form.model) ? form.styleWeight : 0,
    use_negative_prompt: form.useNegativePrompt,
    mask_mode: form.maskMode,
  };

  return {
    prompt: clampPrompt(form.prompt.trim(), h3 ? H3_MAX_PROMPT_LENGTH : MAX_PROMPT_LENGTH),
    negative_prompt: form.negativePrompt.trim(),
    params,
    reference_asset_id: form.referenceAssetId,
    mask_asset_id: form.maskAssetId,
  };
}

/**
 * Validates the form and returns the list of problems, if any.
 *
 * The returned messages are user-facing.
 */
export function validateForm(form: GenerationFormState): string[] {
  const problems: string[] = [];
  const h3 = isH3Model(form.model);
  const maxPrompt = h3 ? H3_MAX_PROMPT_LENGTH : MAX_PROMPT_LENGTH;
  const maxBatch = h3 ? H3_MAX_BATCH_SIZE : MAX_BATCH_SIZE;

  if (form.prompt.trim().length === 0) {
    problems.push('Enter a prompt before generating.');
  } else if (form.prompt.length > maxPrompt) {
    problems.push(
      `The prompt is ${form.prompt.length} characters. This model accepts at most ${maxPrompt}.`,
    );
  }

  if (supportsDimensions(form.model)) {
    if (!isValidDimensionPair(form.width, form.height)) {
      problems.push(
        `Width and height must both be between ${MIN_DIMENSION} and ${MAX_DIMENSION} and a multiple of ${DIMENSION_MULTIPLE}.`,
      );
    }
  }

  if (form.batchSize < 1 || form.batchSize > maxBatch) {
    problems.push(`The batch size must be between 1 and ${maxBatch}.`);
  }

  if (supportsStyle(form.model) && form.styleType && !STYLE_TYPES.includes(form.styleType)) {
    problems.push('Select a valid style preset.');
  }

  if (supportsStyle(form.model) && form.styleType) {
    if (form.styleWeight <= 0 || form.styleWeight > 1) {
      problems.push('The style weight must be greater than 0 and at most 1.');
    }
  }

  return problems;
}

/**
 * Describes a MiniMax status code in user-facing language.
 */
export function describeErrorCode(code: number): string {
  switch (code) {
    case 1002:
      return 'Rate limited by MiniMax. Please wait a moment and retry.';
    case 1004:
      return 'MiniMax rejected the API key. Check the credentials in Settings.';
    case 1008:
      return 'The MiniMax account has insufficient balance. Top up the account and retry.';
    case 1026:
      return 'MiniMax blocked the prompt for sensitive content. Revise the prompt and retry.';
    case 1027:
      return 'MiniMax withheld a generated image because of its content safety check. Revise the prompt and retry.';
    case 2013:
      return 'MiniMax rejected the request parameters. Adjust the generation settings and retry.';
    case 2049:
      return 'The MiniMax API key is invalid. Check the credentials in Settings.';
    case -1:
      return "Aurore could not read MiniMax's response. Open the debug console and share the raw body.";
    default:
      return '';
  }
}

/**
 * Applies the capabilities returned by the backend to a form state.
 *
 * Controls that the selected model does not support are cleared, so the
 * user never submits a value that would be silently ignored.
 */
export function applyCapabilities(
  form: GenerationFormState,
  capabilities: Capabilities,
): GenerationFormState {
  const next: GenerationFormState = { ...form };

  // The aspect ratio and the raw width/height are gated separately,
  // because H3 accepts a ratio plus an area level but no raw size.
  if (capabilities.supports_aspect_ratio) {
    if (!capabilities.aspect_ratios.includes(next.aspectRatio)) {
      next.aspectRatio = (capabilities.aspect_ratios[0] ?? '1:1') as AspectRatio;
    }
  } else {
    next.aspectRatio = '1:1';
  }

  if (capabilities.supports_dimensions) {
    next.width = clampDimension(next.width || capabilities.min_dimension);
    next.height = clampDimension(next.height || capabilities.min_dimension);
  } else {
    next.width = 0;
    next.height = 0;
  }

  // The area level only exists for the models that offer one.
  const areas = capabilities.areas ?? [];
  if (areas.length === 0) {
    next.area = '2K';
  } else if (!areas.includes(next.area)) {
    next.area = (areas.includes('2K') ? '2K' : areas[0]) as ImageArea;
  }

  if (!capabilities.supports_style) {
    next.styleType = '';
    next.styleWeight = 0;
  } else if (next.styleType && !capabilities.style_types.includes(next.styleType)) {
    next.styleType = '';
  }

  // A seed the model cannot accept must not be presented as if it could.
  if (!capabilities.supports_seed) {
    next.seed = 0;
  }
  if (!capabilities.supports_watermark) {
    next.watermark = false;
  }

  next.batchSize = clampBatchSize(next.batchSize, capabilities.max_batch_size);

  return next;
}
