/**
 * The generation parameter panel.
 *
 * This is the capability gate. Controls that MiniMax Image-01 does not
 * support are never rendered at all: there is no CFG scale, no sampler,
 * no inference steps, no scheduler, no LoRA picker and no ControlNet
 * panel anywhere in the application.
 *
 * The two MiniMax model variants expose different parameters, so the
 * dimension and style controls are rendered conditionally based on the
 * capability description returned by the backend.
 */

import {
  ASPECT_RATIOS,
  IMAGE_AREAS,
  MAX_BATCH_SIZE,
  MIN_DIMENSION,
  MAX_DIMENSION,
  MODELS,
  STYLE_TYPES,
} from '../../invoke-adapter/parameters';
import type {
  AspectRatio,
  Capabilities,
  ImageArea,
  MiniMaxModel,
  StyleType,
} from '../../invoke-adapter/types';

interface ParameterPanelProps {
  capabilities: Capabilities;
  model: MiniMaxModel;
  aspectRatio: AspectRatio;
  width: number;
  height: number;
  area: ImageArea;
  batchSize: number;
  seed: number;
  promptOptimizer: boolean;
  watermark: boolean;
  styleType: StyleType | '';
  styleWeight: number;
  onModelChange: (model: MiniMaxModel) => void;
  onAspectRatioChange: (ratio: AspectRatio) => void;
  onWidthChange: (value: number) => void;
  onHeightChange: (value: number) => void;
  onAreaChange: (area: ImageArea) => void;
  onBatchSizeChange: (value: number) => void;
  onSeedChange: (value: number) => void;
  onPromptOptimizerChange: (value: boolean) => void;
  onWatermarkChange: (value: boolean) => void;
  onStyleTypeChange: (value: StyleType | '') => void;
  onStyleWeightChange: (value: number) => void;
  onRandomiseSeed: () => void;
}

/**
 * The parameter panel.
 */
export function ParameterPanel({
  capabilities,
  model,
  aspectRatio,
  width,
  height,
  area,
  batchSize,
  seed,
  promptOptimizer,
  watermark,
  styleType,
  styleWeight,
  onModelChange,
  onAspectRatioChange,
  onWidthChange,
  onHeightChange,
  onAreaChange,
  onBatchSizeChange,
  onSeedChange,
  onPromptOptimizerChange,
  onWatermarkChange,
  onStyleTypeChange,
  onStyleWeightChange,
  onRandomiseSeed,
}: ParameterPanelProps) {
  const showAspectRatio = capabilities.supports_aspect_ratio;
  const showDimensions = capabilities.supports_dimensions;
  const showArea = (capabilities.areas ?? []).length > 0;
  const showStyle = capabilities.supports_style;
  const showSeed = capabilities.supports_seed;
  const showWatermark = capabilities.supports_watermark;
  const maxBatchSize = capabilities.max_batch_size || MAX_BATCH_SIZE;

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Parameters</span>
      </div>
      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="param-model">
            Model
          </label>
          <select
            id="param-model"
            value={model}
            onChange={(event) => onModelChange(event.target.value as MiniMaxModel)}
          >
            {MODELS.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </div>

        {showAspectRatio && (
          <div className="field">
            <label className="field-label" htmlFor="param-aspect">
              Aspect ratio
            </label>
            <select
              id="param-aspect"
              value={aspectRatio}
              onChange={(event) => onAspectRatioChange(event.target.value as AspectRatio)}
            >
              {ASPECT_RATIOS.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
          </div>
        )}

        {showArea && (
          <div className="field">
            <label className="field-label" htmlFor="param-area">
              Output area
            </label>
            <select
              id="param-area"
              value={area}
              onChange={(event) => onAreaChange(event.target.value as ImageArea)}
            >
              {IMAGE_AREAS.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
            <span className="field-hint">
              The model derives the width and height from the aspect ratio and this level. 2K is
              roughly 2048×2048; 4K is four times that.
            </span>
          </div>
        )}

        {showDimensions && (
          <>
            <div className="field">
              <label className="field-label" htmlFor="param-width">
                Width &amp; height
              </label>
              <div className="field-row">
                <input
                  id="param-width"
                  type="number"
                  min={MIN_DIMENSION}
                  max={MAX_DIMENSION}
                  step={8}
                  value={width}
                  onChange={(event) => onWidthChange(Number(event.target.value))}
                />
                <input
                  id="param-height"
                  type="number"
                  min={MIN_DIMENSION}
                  max={MAX_DIMENSION}
                  step={8}
                  value={height}
                  aria-label="Height"
                  onChange={(event) => onHeightChange(Number(event.target.value))}
                />
              </div>
              <span className="field-hint">
                Between {MIN_DIMENSION} and {MAX_DIMENSION}, in multiples of 8. The aspect ratio takes
                precedence when both are sent.
              </span>
            </div>
          </>
        )}

        {showStyle && (
          <>
            <div className="field">
              <label className="field-label" htmlFor="param-style">
                Style preset
              </label>
              <select
                id="param-style"
                value={styleType}
                onChange={(event) => onStyleTypeChange(event.target.value as StyleType | '')}
              >
                <option value="">None</option>
                {STYLE_TYPES.map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </div>

            {styleType !== '' && (
              <div className="field">
                <label className="field-label" htmlFor="param-style-weight">
                  Style weight
                  <span className="text-dim">{styleWeight.toFixed(2)}</span>
                </label>
                <input
                  id="param-style-weight"
                  type="range"
                  min={0.05}
                  max={1}
                  step={0.05}
                  value={styleWeight}
                  onChange={(event) => onStyleWeightChange(Number(event.target.value))}
                />
              </div>
            )}
          </>
        )}

        <div className="field-row">
          <div className="field">
            <label className="field-label" htmlFor="param-batch">
              Batch size
            </label>
            <input
              id="param-batch"
              type="number"
              min={1}
              max={maxBatchSize}
              value={batchSize}
              onChange={(event) => onBatchSizeChange(Number(event.target.value))}
            />
          </div>

          {showSeed && (
            <div className="field">
              <label className="field-label" htmlFor="param-seed">
                Seed
              </label>
              <div className="field-row">
                <input
                  id="param-seed"
                  type="number"
                  min={0}
                  value={seed}
                  placeholder="random"
                  onChange={(event) => onSeedChange(Number(event.target.value))}
                />
                <button
                  type="button"
                  className="btn btn-sm"
                  style={{ flex: '0 0 auto' }}
                  onClick={onRandomiseSeed}
                >
                  Random
                </button>
              </div>
            </div>
          )}
        </div>

        <label className="checkbox">
          <input
            type="checkbox"
            checked={promptOptimizer}
            onChange={(event) => onPromptOptimizerChange(event.target.checked)}
          />
          <span className="checkbox-text">
            <span className="checkbox-title">Prompt optimiser</span>
            <span className="field-hint">Let MiniMax expand the prompt before generating.</span>
          </span>
        </label>

        {showWatermark && (
          <label className="checkbox">
            <input
              type="checkbox"
              checked={watermark}
              onChange={(event) => onWatermarkChange(event.target.checked)}
            />
            <span className="checkbox-text">
              <span className="checkbox-title">AI watermark</span>
              <span className="field-hint">Adds the MiniMax generated-content watermark.</span>
            </span>
          </label>
        )}

        {!showAspectRatio && (
          <div className="notice">
            <span className="mono">image-01-live</span> does not accept an aspect ratio or
            dimensions. Switch to another model to control the output size.
          </div>
        )}

        {showStyle && (
          <div className="notice">
            Style presets apply only to <span className="mono">image-01-live</span>.
          </div>
        )}
      </div>
    </section>
  );
}
