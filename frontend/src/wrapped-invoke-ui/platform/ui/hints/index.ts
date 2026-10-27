// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   platform/ui/hints/index.ts
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

// Keep the hints entry outside the UI barrel to preserve its direct-importer budget.
export * from './FeatureHint';
export { FEATURE_HINT_FIELD_PROPS } from './hintGeometry';
export * from './hintRegistry';
export * from './hintsContext';
