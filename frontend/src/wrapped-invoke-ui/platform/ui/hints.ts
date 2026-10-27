// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// Local replacement for InvokeAI's @platform/ui/hints module.
//
// InvokeAI uses these hints to annotate controls that are temporarily
// unavailable, for example while a model is loading. Aurore keeps the
// same shape so the vendored components behave identically.

/** The hint shown next to a control. */
export interface Hint {
  /** The hint text. */
  content: string;
  /** The hint tone. */
  tone?: 'info' | 'warning' | 'error' | 'success';
}

/**
 * Builds a hint descriptor for a control.
 */
export const hint = (content: string, tone: Hint['tone'] = 'info'): Hint => ({
  content,
  tone,
});
