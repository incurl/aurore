// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   features/queue/ui/QueueStatusDot.tsx
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import type { QueueItemStatus } from '@wrapped/features/queue/core/types';

import { Box } from '@chakra-ui/react';
import { useTranslation } from 'react-i18next';

import { getStatusMeta } from './statusMeta';

const ACTIVE_HALO = { boxShadow: '0 0 0 3px {colors.accent.subtle}' } as const;

/** Small status indicator dot; the running state gets a soft accent halo. */
export const QueueStatusDot = ({ status, boxSize = '2' }: { status: QueueItemStatus; boxSize?: string }) => {
  const { t } = useTranslation();
  const { dotColor, labelKey } = getStatusMeta(status);

  return (
    <Box
      aria-label={t(labelKey)}
      bg={dotColor}
      boxSize={boxSize}
      flexShrink={0}
      rounded="full"
      role="img"
      css={status === 'in_progress' ? ACTIVE_HALO : undefined}
    />
  );
};
