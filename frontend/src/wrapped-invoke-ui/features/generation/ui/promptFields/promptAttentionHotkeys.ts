// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   features/generation/ui/promptFields/promptAttentionHotkeys.ts
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import type { PromptAttentionDirection } from '@wrapped/features/generation/core/prompt/attention';

import { adjustPromptAttention } from '@wrapped/features/generation/core/prompt/attention';

/** `PromptTextarea` writes it only while the model reads attention syntax; the hotkey catalog claims keys on it. */
const PROMPT_ATTENTION_TARGET_ATTR = 'data-prompt-attention-target';
export const PROMPT_ATTENTION_TARGET_PROPS = { [PROMPT_ATTENTION_TARGET_ATTR]: 'true' } as const;

/** Guard readOnly explicitly: native setters and synthetic input bypass it and could merge templates twice. */
const isPromptAttentionTarget = (element: Element | null): element is HTMLTextAreaElement =>
  element instanceof HTMLTextAreaElement &&
  element.getAttribute(PROMPT_ATTENTION_TARGET_ATTR) === 'true' &&
  !element.readOnly;

const setTextareaValue = (textarea: HTMLTextAreaElement, value: string): void => {
  const valueSetter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set;

  if (valueSetter) {
    valueSetter.call(textarea, value);
  } else {
    textarea.value = value;
  }

  textarea.dispatchEvent(new Event('input', { bubbles: true }));
};

const replaceTextareaValue = (textarea: HTMLTextAreaElement, value: string): void => {
  textarea.focus();
  textarea.setSelectionRange(0, textarea.value.length);

  if (typeof document.execCommand === 'function' && document.execCommand('insertText', false, value)) {
    return;
  }

  setTextareaValue(textarea, value);
};

export const adjustFocusedPromptAttention = (
  direction: PromptAttentionDirection,
  preferNumericAttentionStyle: boolean
): boolean => {
  const textarea = document.activeElement;

  if (!isPromptAttentionTarget(textarea)) {
    return false;
  }

  const selectionDirection = textarea.selectionDirection;
  const result = adjustPromptAttention(
    textarea.value,
    textarea.selectionStart,
    textarea.selectionEnd,
    direction,
    preferNumericAttentionStyle
  );

  if (result.prompt === textarea.value) {
    return false;
  }

  replaceTextareaValue(textarea, result.prompt);
  textarea.setSelectionRange(result.selectionStart, result.selectionEnd, selectionDirection);

  return true;
};
