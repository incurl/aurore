// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// Local replacement for InvokeAI's @platform/ui/Menu.
//
// The vendored SeedInput uses a Chakra menu with a portal. This wrapper
// exposes the same surface so the vendored component can be reused
// unchanged.
//
// InvokeAI's own Menu keeps a registry of open menus so that a floating
// widget can dismiss them. Aurore has no floating widgets, so that
// coupling is dropped.
//
// Chakra v3 exposes the menu as a namespace of parts, so the wrapper
// composes Root, Trigger, Positioner and Content itself.

import { Menu as ChakraMenu, Portal } from '@chakra-ui/react';
import type { ComponentProps, ReactNode } from 'react';

export { Portal };

/**
 * The application menu.
 */
export const Menu = {
  Root: ChakraMenu.Root,
  Trigger: ChakraMenu.Trigger,
  Indicator: ChakraMenu.Indicator,
};

/**
 * The portaled menu content.
 */
export interface MenuContentProps extends ComponentProps<typeof ChakraMenu.Content> {
  /** The menu items. */
  children?: ReactNode;
}

export const MenuContent = ({ children, ...props }: MenuContentProps) => (
  <Portal>
    <ChakraMenu.Positioner>
      <ChakraMenu.Content {...props}>{children}</ChakraMenu.Content>
    </ChakraMenu.Positioner>
  </Portal>
);
