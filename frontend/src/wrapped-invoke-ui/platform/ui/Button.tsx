// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// This file is a trimmed adaptation of the InvokeAI web frontend
// (https://github.com/invoke-ai/InvokeAI), file:
//
//   platform/ui/Button.tsx
//
// It has been adapted for Aurore: InvokeAI's runtime-coupled imports
// have been replaced with local equivalents, and the component now
// targets the MiniMax Image-01 backend.
//
// InvokeAI is used here purely as a UI component library. None of its
// native diffusion backend, model runtime or workflow engine is
// included or invoked.

import type { ComponentProps, ReactNode } from 'react';

import {
  Button as ChakraButton,
  CloseButton as ChakraCloseButton,
  Icon,
  IconButton as ChakraIconButton,
} from '@chakra-ui/react';
import { useCallback } from 'react';

import { Tooltip } from './Tooltip';

export type ButtonProps = ComponentProps<typeof ChakraButton>;
export type CloseButtonProps = ComponentProps<typeof ChakraCloseButton>;
type ChakraIconButtonProps = ComponentProps<typeof ChakraIconButton>;

type IconButtonAccessibleName =
  | {
      'aria-label': string;
      'aria-labelledby'?: never;
    }
  | {
      'aria-label'?: never;
      'aria-labelledby': string;
    };

export type IconButtonProps = Omit<ChakraIconButtonProps, 'aria-label' | 'aria-labelledby'> & IconButtonAccessibleName;

/** Solid defaults to accent; other variants to gray. Explicit colorPalette overrides both. */
const defaultPalette = (variant: ButtonProps['variant']): ButtonProps['colorPalette'] =>
  variant === undefined || variant === 'solid' ? 'accent' : 'gray';

export const Button = ({ colorPalette, ...props }: ButtonProps) => (
  <ChakraButton colorPalette={colorPalette ?? defaultPalette(props.variant)} {...props} />
);

export const IconButton = ({ colorPalette, ...props }: IconButtonProps) => (
  <ChakraIconButton colorPalette={colorPalette ?? defaultPalette(props.variant)} {...props} />
);

/** Dismissal chrome is muted. */
export const CloseButton = (props: CloseButtonProps) => <ChakraCloseButton color="fg.muted" {...props} />;

export interface ToggleIconButtonProps extends Omit<
  IconButtonProps,
  'aria-label' | 'aria-labelledby' | 'aria-pressed' | 'children' | 'onClick'
> {
  checked: boolean;
  icon: React.ElementType;
  /** Names the control for assistive tech and, unless overridden, the tooltip. */
  label: string;
  tooltip?: ReactNode;
  onCheckedChange: (checked: boolean) => void;
}

/** Use an icon toggle when nearby context cannot explain ToggleDot; aria-pressed carries state. */
export const ToggleIconButton = ({
  checked,
  icon,
  label,
  onCheckedChange,
  tooltip = label,
  ...props
}: ToggleIconButtonProps) => {
  const handleClick = useCallback(() => onCheckedChange(!checked), [checked, onCheckedChange]);

  return (
    <Tooltip content={tooltip}>
      <IconButton
        aria-label={label}
        aria-pressed={checked}
        color={checked ? undefined : 'fg.muted'}
        size="sm"
        variant={checked ? 'solid' : 'ghost'}
        {...props}
        onClick={handleClick}
      >
        <Icon as={icon} boxSize="3.5" />
      </IconButton>
    </Tooltip>
  );
};
