// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   features/queue/ui/statusMeta.ts
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import type { QueueItemStatus } from '@wrapped/features/queue/core/types';

export interface QueueStatusMeta {
  labelKey: string;
  colorPalette: string;
  dotColor: string;
}

const STATUS_META: Record<QueueItemStatus, QueueStatusMeta> = {
  canceled: { colorPalette: 'orange', dotColor: 'orange.solid', labelKey: 'common.status.canceled' },
  completed: { colorPalette: 'green', dotColor: 'green.solid', labelKey: 'common.status.completed' },
  failed: { colorPalette: 'red', dotColor: 'red.solid', labelKey: 'common.status.failed' },
  in_progress: { colorPalette: 'accent', dotColor: 'accent.solid', labelKey: 'common.generating' },
  pending: { colorPalette: 'gray', dotColor: 'fg.muted', labelKey: 'common.status.pending' },
  waiting: { colorPalette: 'gray', dotColor: 'fg.muted', labelKey: 'common.status.waiting' },
};

export const getStatusMeta = (status: QueueItemStatus): QueueStatusMeta => STATUS_META[status];
