// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   platform/ui/hints/hintsContext.tsx
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import type { ReactNode } from 'react';

import { createContext, use } from 'react';

/** App supplies hint preferences/actions because Platform cannot import Workbench. */
export interface FeatureHintsAdapter {
  enabled: boolean;
  /** Turns hints off from inside a card; null when the host cannot persist preferences. */
  onDisable: (() => void) | null;
}

/** Default to disabled without a provider so isolated trees remain unchanged. */
const DEFAULT_FEATURE_HINTS_ADAPTER: FeatureHintsAdapter = {
  enabled: false,
  onDisable: null,
};

const FeatureHintsContext = createContext<FeatureHintsAdapter>(DEFAULT_FEATURE_HINTS_ADAPTER);

export const FeatureHintsProvider = ({ adapter, children }: { adapter: FeatureHintsAdapter; children: ReactNode }) => (
  <FeatureHintsContext value={adapter}>{children}</FeatureHintsContext>
);

export const useFeatureHints = (): FeatureHintsAdapter => use(FeatureHintsContext);
