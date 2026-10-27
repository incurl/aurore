/**
 * The reference image and mask upload panel.
 *
 * Preserves the InvokeAI reference image upload experience. Uploaded
 * files are stored by PocketBase's native file field and are sent to
 * MiniMax as base64 data URLs, because a desktop application has no
 * public URL to share.
 */

import { useRef } from 'react';

import type { Asset } from '../../invoke-adapter/types';

interface AssetPanelProps {
  assets: Asset[];
  referenceAssetId: string;
  maskAssetId: string;
  onUpload: (kind: 'reference' | 'mask', file: File) => void;
  onSelect: (kind: 'reference' | 'mask', assetId: string) => void;
  onRemove: (kind: 'reference' | 'mask', assetId: string) => void;
}

/**
 * The asset upload panel.
 */
export function AssetPanel({
  assets,
  referenceAssetId,
  maskAssetId,
  onUpload,
  onSelect,
  onRemove,
}: AssetPanelProps) {
  const referenceInput = useRef<HTMLInputElement>(null);
  const maskInput = useRef<HTMLInputElement>(null);

  const references = assets.filter((asset) => asset.kind === 'reference');
  const masks = assets.filter((asset) => asset.kind === 'mask');

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Reference &amp; mask</span>
      </div>
      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="asset-reference">
            Reference image
          </label>
          <input
            id="asset-reference"
            ref={referenceInput}
            type="file"
            accept="image/png,image/jpeg"
            style={{ display: 'none' }}
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (file) {
                onUpload('reference', file);
              }
              event.target.value = '';
            }}
          />
          <button
            type="button"
            className="btn btn-block"
            onClick={() => referenceInput.current?.click()}
          >
            Upload reference image
          </button>
          <span className="field-hint">
            Used for image-to-image generation. MiniMax accepts a single character reference, sent
            as a base64 data URL.
          </span>
          {references.length > 0 && (
            <div className="asset-list">
              {references.map((asset) => (
                <div key={asset.id} className="asset-item">
                  <img className="asset-thumb" src={asset.url} alt="Reference" />
                  <span className="asset-name">
                    {asset.width}×{asset.height}
                  </span>
                  <button
                    type="button"
                    className="btn btn-sm"
                    onClick={() => onSelect('reference', asset.id)}
                    disabled={referenceAssetId === asset.id}
                  >
                    {referenceAssetId === asset.id ? 'Selected' : 'Use'}
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm btn-danger"
                    onClick={() => onRemove('reference', asset.id)}
                    aria-label="Remove reference"
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="field">
          <label className="field-label" htmlFor="asset-mask">
            Mask image
          </label>
          <input
            id="asset-mask"
            ref={maskInput}
            type="file"
            accept="image/png,image/jpeg"
            style={{ display: 'none' }}
            onChange={(event) => {
              const file = event.target.files?.[0];
              if (file) {
                onUpload('mask', file);
              }
              event.target.value = '';
            }}
          />
          <button type="button" className="btn btn-block" onClick={() => maskInput.current?.click()}>
            Upload mask
          </button>
          <span className="field-hint">
            MiniMax has no alpha mask input. The mask is applied locally to the returned image.
          </span>
          {masks.length > 0 && (
            <div className="asset-list">
              {masks.map((asset) => (
                <div key={asset.id} className="asset-item">
                  <img className="asset-thumb" src={asset.url} alt="Mask" />
                  <span className="asset-name">
                    {asset.width}×{asset.height}
                  </span>
                  <button
                    type="button"
                    className="btn btn-sm"
                    onClick={() => onSelect('mask', asset.id)}
                    disabled={maskAssetId === asset.id}
                  >
                    {maskAssetId === asset.id ? 'Selected' : 'Use'}
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm btn-danger"
                    onClick={() => onRemove('mask', asset.id)}
                    aria-label="Remove mask"
                  >
                    ×
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
