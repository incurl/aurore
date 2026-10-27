/**
 * Types describing the Invoke-style UI payload and the MiniMax request
 * that Aurore derives from it.
 *
 * The UI payload mirrors the shape of InvokeAI's generation form, but only
 * carries fields that MiniMax Image-01 can actually honour. Unsupported
 * diffusion controls (CFG scale, sampler, steps, scheduler, LoRA,
 * ControlNet) are deliberately absent from both types, so they can never
 * be sent.
 */

/** MiniMax image models. */
export type MiniMaxModel = 'image-01' | 'image-01-live';

/** MiniMax aspect ratios. */
export type AspectRatio =
  | '1:1'
  | '16:9'
  | '4:3'
  | '3:2'
  | '2:3'
  | '3:4'
  | '9:16'
  | '21:9';

/** MiniMax style presets. Only valid for `image-01-live`. */
export type StyleType = '漫画' | '元气' | '中世纪' | '水彩';

/** How a locally applied mask is combined with the generated image. */
export type MaskMode = 'generated_in_mask' | 'generated_outside_mask';

/** The subject reference kind. The API documents only `character`. */
export type SubjectReferenceType = 'character';

/**
 * The generation form state, in the shape the Invoke-style UI produces.
 */
export interface GenerationFormState {
  /** Positive prompt. */
  prompt: string;
  /** Negative prompt. MiniMax has no such field, so it is folded in. */
  negativePrompt: string;
  /** The selected model. */
  model: MiniMaxModel;
  /** The selected aspect ratio. Only valid for `image-01`. */
  aspectRatio: AspectRatio;
  /** Explicit output width. Only valid for `image-01`. */
  width: number;
  /** Explicit output height. Only valid for `image-01`. */
  height: number;
  /** Number of images to generate (1-9). */
  batchSize: number;
  /** Requested seed. Zero means the server picks a random seed. */
  seed: number;
  /** Enables the MiniMax prompt optimiser. */
  promptOptimizer: boolean;
  /** Enables the MiniMax AI watermark. */
  watermark: boolean;
  /** The selected style preset. Only valid for `image-01-live`. */
  styleType: StyleType | '';
  /** The style strength in (0, 1]. */
  styleWeight: number;
  /**
   * Appends the negative prompt to the positive prompt.
   *
   * MiniMax Image-01 has no negative_prompt field, so this is how the
   * user's negative prompt reaches the model at all.
   */
  useNegativePrompt: boolean;
  /** The selected mask composite mode. */
  maskMode: MaskMode;
  /** The uploaded reference image asset id, if any. */
  referenceAssetId: string;
  /** The uploaded mask asset id, if any. */
  maskAssetId: string;
}

/**
 * The request shape that is sent to the Go backend.
 *
 * It mirrors the Go `jobs.Request` type.
 */
export interface GenerationRequest {
  prompt: string;
  negative_prompt: string;
  params: {
    model: string;
    aspect_ratio: string;
    width: number;
    height: number;
    batch_size: number;
    seed: number;
    prompt_optimizer: boolean;
    watermark: boolean;
    style_type: string;
    style_weight: number;
    use_negative_prompt: boolean;
    mask_mode: string;
  };
  reference_asset_id: string;
  mask_asset_id: string;
}

/*
 * The job, image, asset and preset shapes are defined by the Go backend
 * and generated into TypeScript by `wails3 generate bindings -ts`. They
 * are re-exported here so that the adapter and the React components can
 * import every boundary type from a single place.
 *
 * The status and kind fields are widened to literal unions so the UI can
 * switch on them exhaustively.
 */

import type {
  Asset,
  Capabilities,
  Image,
  Job,
  Preset,
} from '../wails-bindings';

/** The job lifecycle states. */
export type JobStatus = Job['status'];

/** The kinds of uploaded asset. */
export type AssetKind = Asset['kind'];

/** A generation job as returned by the Go backend. */
export type { Job, Image, Asset, Preset, Capabilities };

/**
 * The application settings as seen by the UI.
 *
 * The API key is never returned by the backend, only whether one is set.
 */
export type Settings = {
  has_api_key: boolean;
  base_url: string;
  model: string;
  defaults: {
    model: string;
    aspect_ratio: string;
    width: number;
    height: number;
    batch_size: number;
    seed: number;
    prompt_optimizer: boolean;
    watermark: boolean;
    style_type: string;
    style_weight: number;
  };
  ui: UISettings;
};

/**
 * The interface preferences.
 */
export interface UISettings {
  /** The interface scale, as a percentage of the base size (50-200). */
  zoom: number;
  /** The interface colour scheme: 'dark' or 'light'. */
  theme: string;
}
