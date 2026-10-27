/**
 * The prompt editor.
 *
 * This preserves the InvokeAI prompt editing experience: a positive
 * prompt, a negative prompt and a character counter that reflects the
 * MiniMax limit.
 *
 * The negative prompt is labelled clearly because MiniMax Image-01 has no
 * negative_prompt field: the value is folded into the positive prompt
 * before the request is sent.
 */

import { useMemo } from 'react';

import { MAX_PROMPT_LENGTH } from '../../invoke-adapter/parameters';

interface PromptEditorProps {
  prompt: string;
  negativePrompt: string;
  useNegativePrompt: boolean;
  onPromptChange: (value: string) => void;
  onNegativePromptChange: (value: string) => void;
  onUseNegativePromptChange: (value: boolean) => void;
}

/**
 * The prompt editor panel.
 */
export function PromptEditor({
  prompt,
  negativePrompt,
  useNegativePrompt,
  onPromptChange,
  onNegativePromptChange,
  onUseNegativePromptChange,
}: PromptEditorProps) {
  const remaining = useMemo(() => MAX_PROMPT_LENGTH - prompt.length, [prompt.length]);
  const overLimit = remaining < 0;

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Prompt</span>
        <span className={overLimit ? 'text-muted' : 'text-dim'}>
          {prompt.length} / {MAX_PROMPT_LENGTH}
        </span>
      </div>
      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="prompt-positive">
            Positive prompt
          </label>
          <textarea
            id="prompt-positive"
            rows={7}
            value={prompt}
            placeholder="A man in a white t-shirt, full-body, standing front view, outdoors, with the Venice Beach sign in the background…"
            onChange={(event) => onPromptChange(event.target.value)}
          />
        </div>

        <div className="field">
          <label className="field-label" htmlFor="prompt-negative">
            Negative prompt
          </label>
          <textarea
            id="prompt-negative"
            rows={3}
            value={negativePrompt}
            placeholder="blurry, low quality, distorted hands"
            onChange={(event) => onNegativePromptChange(event.target.value)}
          />
          <label className="checkbox">
            <input
              type="checkbox"
              checked={useNegativePrompt}
              onChange={(event) => onUseNegativePromptChange(event.target.checked)}
            />
            <span className="checkbox-text">
              <span className="checkbox-title">Append to the positive prompt</span>
              <span className="field-hint">
                MiniMax Image-01 has no negative prompt field. When enabled, this text is appended to
                the positive prompt as <span className="mono">Avoid: …</span>
              </span>
            </span>
          </label>
        </div>
      </div>
    </section>
  );
}
