/**
 * The prompt preset panel.
 *
 * Preserves the InvokeAI prompt preset UX: save the current prompt and
 * parameters under a name, then load it back.
 */

import { useState } from 'react';

import type { Preset } from '../../invoke-adapter/types';

interface PresetPanelProps {
  presets: Preset[];
  onSave: (name: string) => void;
  onLoad: (preset: Preset) => void;
  onDelete: (presetId: string) => void;
}

/**
 * The preset panel.
 */
export function PresetPanel({ presets, onSave, onLoad, onDelete }: PresetPanelProps) {
  const [name, setName] = useState('');

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Presets</span>
      </div>
      <div className="panel-body">
        <div className="field-row">
          <input
            type="text"
            placeholder="Preset name"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
          <button
            type="button"
            className="btn"
            style={{ flex: '0 0 auto' }}
            disabled={name.trim().length === 0}
            onClick={() => {
              onSave(name.trim());
              setName('');
            }}
          >
            Save
          </button>
        </div>

        {presets.length === 0 ? (
          <div className="empty-state">Save a prompt to reuse it later.</div>
        ) : (
          <div className="preset-list">
            {presets.map((preset) => (
              <div key={preset.id} className="preset-item">
                <button type="button" className="preset-name" onClick={() => onLoad(preset)}>
                  {preset.name}
                </button>
                <button
                  type="button"
                  className="btn btn-sm btn-danger"
                  onClick={() => onDelete(preset.id)}
                  aria-label={`Delete preset ${preset.name}`}
                >
                  ×
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
