// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// Local replacement for InvokeAI's @platform/ui/Tooltip.
//
// The vendored components render tooltips through this wrapper. Aurore
// provides a Chakra v3 tooltip with the same trigger-id contract.
//
// InvokeAI's own Tooltip registers the trigger with its widget overlay
// registry so that a floating widget can dismiss it. Aurore has no
// floating widgets, so that coupling is dropped.
//
// Chakra v3 exposes the tooltip as a namespace of parts, so the wrapper
// composes Root, Trigger and Positioner/Content itself.

import { Tooltip as ChakraTooltip } from '@chakra-ui/react';
import type { ReactElement, ReactNode } from 'react';

export interface TooltipProps {
  /** The tooltip content. */
  content: ReactNode;
  /** The element the tooltip is attached to. */
  children: ReactElement;
  /** The ids shared with the trigger, for accessibility. */
  ids?: Record<string, string>;
}

/**
 * The application tooltip.
 */
export const Tooltip = ({ content, children, ids }: TooltipProps) => (
  <ChakraTooltip.Root ids={ids} openDelay={300} lazyMount>
    <ChakraTooltip.Trigger asChild>{children}</ChakraTooltip.Trigger>
    <ChakraTooltip.Positioner>
      <ChakraTooltip.Content>{content}</ChakraTooltip.Content>
    </ChakraTooltip.Positioner>
  </ChakraTooltip.Root>
);
