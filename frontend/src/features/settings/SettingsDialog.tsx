/**
 * The settings dialog.
 *
 * Holds the MiniMax credentials and the default generation preferences.
 *
 * The API key is write-only from the UI's point of view: the backend
 * never returns it, it is stored encrypted in PocketBase and only
 * decrypted by the Go process.
 */

import { useEffect, useState } from 'react';

import type { Settings } from '../../invoke-adapter/types';

interface SettingsDialogProps {
  open: boolean;
  settings: Settings | null;
  onClose: () => void;
  onSave: (apiKey: string, baseUrl: string, model: string) => void;
  onTest: () => void;
  onSaveDefaults: (defaults: Settings['defaults']) => void;
}

/**
 * The settings dialog.
 */
export function SettingsDialog({
  open,
  settings,
  onClose,
  onSave,
  onTest,
  onSaveDefaults,
}: SettingsDialogProps) {
  const [apiKey, setApiKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [model, setModel] = useState('image-01');
  const [batchSize, setBatchSize] = useState(1);
  const [aspectRatio, setAspectRatio] = useState('1:1');

  useEffect(() => {
    if (!settings) {
      return;
    }
    setBaseUrl(settings.base_url);
    setModel(settings.model);
    setBatchSize(settings.defaults.batch_size);
    setAspectRatio(settings.defaults.aspect_ratio);
  }, [settings]);

  if (!open) {
    return null;
  }

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Settings">
      <div className="modal">
        <div className="modal-header">
          <span>Settings</span>
          <button type="button" className="btn btn-sm" onClick={onClose}>
            Close
          </button>
        </div>

        <div className="modal-body">
          <div className="field">
            <label className="field-label" htmlFor="settings-key">
              MiniMax API key
            </label>
            <input
              id="settings-key"
              type="password"
              value={apiKey}
              placeholder={settings?.has_api_key ? '•••••••• (saved)' : 'Paste your API key'}
              autoComplete="off"
              onChange={(event) => setApiKey(event.target.value)}
            />
            <span className="field-hint">
              Stored encrypted in the local database. Leave empty to keep the current key.
            </span>
          </div>

          <div className="field">
            <label className="field-label" htmlFor="settings-base-url">
              API base URL
            </label>
            <input
              id="settings-base-url"
              type="text"
              value={baseUrl}
              onChange={(event) => setBaseUrl(event.target.value)}
            />
          </div>

          <div className="field">
            <label className="field-label" htmlFor="settings-model">
              Default model
            </label>
            <select
              id="settings-model"
              value={model}
              onChange={(event) => setModel(event.target.value)}
            >
              <option value="image-01">image-01</option>
              <option value="image-01-live">image-01-live</option>
            </select>
          </div>

          <div className="field-row">
            <div className="field">
              <label className="field-label" htmlFor="settings-batch">
                Default batch size
              </label>
              <input
                id="settings-batch"
                type="number"
                min={1}
                max={9}
                value={batchSize}
                onChange={(event) => setBatchSize(Number(event.target.value))}
              />
            </div>
            <div className="field">
              <label className="field-label" htmlFor="settings-aspect">
                Default aspect ratio
              </label>
              <select
                id="settings-aspect"
                value={aspectRatio}
                onChange={(event) => setAspectRatio(event.target.value)}
              >
                {['1:1', '16:9', '4:3', '3:2', '2:3', '3:4', '9:16', '21:9'].map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        <div className="modal-footer">
          <button
            type="button"
            className="btn"
            onClick={() =>
              onSaveDefaults({
                ...(settings?.defaults ?? {
                  model,
                  aspect_ratio: aspectRatio,
                  width: 1024,
                  height: 1024,
                  batch_size: batchSize,
                  seed: 0,
                  prompt_optimizer: false,
                  watermark: false,
                  style_type: '',
                  style_weight: 0.8,
                }),
                model,
                aspect_ratio: aspectRatio,
                batch_size: batchSize,
              })
            }
          >
            Save defaults
          </button>
          <button type="button" className="btn" onClick={onTest}>
            Test connection
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => {
              onSave(apiKey, baseUrl, model);
              setApiKey('');
            }}
          >
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
