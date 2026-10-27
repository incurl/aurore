import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

import { describe, expect, it } from 'vitest';

import { toGenerationRequest, DEFAULT_FORM_STATE } from './parameters';
import type { GenerationFormState } from './types';

/**
 * The controls that MiniMax Image-01 does not support.
 *
 * Acceptance criterion: no unsupported diffusion control may be visible
 * to the end user. The test asserts both that the adapter never emits
 * them and that the UI never renders them.
 */
const UNSUPPORTED_CONTROLS = [
  'cfg_scale',
  'cfgScale',
  'sampler',
  'scheduler',
  'steps',
  'inference_steps',
  'denoising_strength',
  'lora',
  'controlnet',
  'control_net',
];

/**
 * Recursively lists the source files of a directory.
 */
function sourceFiles(directory: string): string[] {
  const files: string[] = [];

  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry);
    if (statSync(path).isDirectory()) {
      files.push(...sourceFiles(path));
    } else if (/\.(ts|tsx)$/.test(entry)) {
      files.push(path);
    }
  }

  return files;
}

describe('unsupported control compliance', () => {
  it('never emits an unsupported control in the request', () => {
    const form: GenerationFormState = {
      ...DEFAULT_FORM_STATE,
      prompt: 'a red bicycle',
    };
    const serialised = JSON.stringify(toGenerationRequest(form));

    for (const control of UNSUPPORTED_CONTROLS) {
      expect(serialised).not.toContain(control);
    }
  });

  it('does not render an unsupported control in the application UI', () => {
    // The vendored InvokeAI tree is excluded: it is upstream code that
    // is not reachable from Aurore's own panels.
    const directories = ['src/features', 'src/wails-bindings', 'src/App.tsx'];
    const files = directories.flatMap((entry) => {
      const path = join(process.cwd(), entry);
      try {
        return statSync(path).isDirectory() ? sourceFiles(path) : [path];
      } catch {
        return [];
      }
    });

    expect(files.length).toBeGreaterThan(0);

    for (const file of files) {
      const source = readFileSync(file, 'utf8').toLowerCase();

      for (const control of UNSUPPORTED_CONTROLS) {
        // The check is for user-visible labels, so it looks for the
        // control in a JSX string or a display name.
        const pattern = new RegExp(`['"\`][^'"\`]*\\b${control.toLowerCase()}\\b[^'"\`]*['"\`]`);
        expect(
          pattern.test(source),
          `${file} must not reference the unsupported control "${control}"`,
        ).toBe(false);
      }
    }
  });

  it('does not import the InvokeAI node or workflow features', () => {
    const files = sourceFiles(join(process.cwd(), 'src/features'));

    for (const file of files) {
      const source = readFileSync(file, 'utf8');
      expect(source).not.toMatch(/features\/(nodes|workflow)/);
      expect(source).not.toMatch(/@xyflow|ReactFlow/);
    }
  });
});
