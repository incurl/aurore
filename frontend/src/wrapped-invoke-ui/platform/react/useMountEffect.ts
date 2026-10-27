// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   platform/react/useMountEffect.ts
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import { useEffect, type EffectCallback } from 'react';

/** Explicit escape hatch for one-time external registration. */
export const useMountEffect = (effect: EffectCallback): void => {
  // The empty dependency list is the contract of this hook.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(effect, []);
};
