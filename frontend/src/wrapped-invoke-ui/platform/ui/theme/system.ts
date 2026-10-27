// Copyright (c) 2022-2026 InvokeAI Contributors
// SPDX-License-Identifier: Apache-2.0
//
// The Aurore Chakra theme.
//
// InvokeAI's design system is built on Chakra v3 semantic tokens: the
// accent palette, the foreground/background scales, the border scale and
// the control radius. The vendored components reference those tokens
// directly, so Aurore defines the same semantic layer.
//
// The palette is Aurore's own dark theme, tuned to match the rest of the
// application stylesheet.

import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react';

/** The Chakra configuration with the semantic tokens. */
export const config = defineConfig({
  globalCss: {
    'html, body, #root': {
      height: '100%',
    },
    body: {
      bg: 'bg.base',
      color: 'fg.muted',
      fontFamily: 'Inter, system-ui, sans-serif',
    },
  },
  theme: {
    tokens: {
      colors: {
        accent: {
          solid: { value: '#6366f1' },
          contrast: { value: '#ffffff' },
          fg: { value: '#a5b4fc' },
          muted: { value: '#4c4ea8' },
          subtle: { value: '#1e1f3d' },
          emphasized: { value: '#2a2b52' },
          focusRing: { value: '#818cf8' },
        },
        fg: {
          muted: { value: '#9a9aa8' },
          subtle: { value: '#6d6d7c' },
          inverted: { value: '#101014' },
        },
        bg: {
          base: { value: '#101014' },
          raised: { value: '#17171d' },
          inset: { value: '#0b0b0e' },
          panel: { value: '#17171d' },
          muted: { value: '#1e1e26' },
          subtle: { value: '#1a1a21' },
          hover: { value: '#26262f' },
          emphasized: { value: '#2c2c36' },
          error: { value: '#3b1d1d' },
          warning: { value: '#3a2f14' },
          success: { value: '#14331f' },
        },
        border: {
          subtle: { value: '#2c2c36' },
          emphasized: { value: '#3a3a46' },
          image: { value: '#3a3a46' },
          error: { value: '#7f3a3a' },
        },
      },
      radii: {
        control: { value: '0.375rem' },
      },
    },
    semanticTokens: {
      colors: {
        'accent.solid': { value: '{colors.accent.solid}' },
        'accent.contrast': { value: '{colors.accent.contrast}' },
        'accent.fg': { value: '{colors.accent.fg}' },
        'accent.muted': { value: '{colors.accent.muted}' },
        'accent.subtle': { value: '{colors.accent.subtle}' },
        'accent.emphasized': { value: '{colors.accent.emphasized}' },
        'accent.focusRing': { value: '{colors.accent.focusRing}' },

        'fg.muted': { value: '{colors.fg.muted}' },
        'fg.subtle': { value: '{colors.fg.subtle}' },
        'fg.inverted': { value: '{colors.fg.inverted}' },
        'fg.error': { value: '{colors.fg.error}' },
        'fg.warning': { value: '{colors.fg.warning}' },
        'fg.success': { value: '{colors.fg.success}' },

        'bg.base': { value: '{colors.bg.base}' },
        'bg.raised': { value: '{colors.bg.raised}' },
        'bg.inset': { value: '{colors.bg.inset}' },
        'bg.panel': { value: '{colors.bg.panel}' },
        'bg.muted': { value: '{colors.bg.muted}' },
        'bg.subtle': { value: '{colors.bg.subtle}' },
        'bg.hover': { value: '{colors.bg.hover}' },
        'bg.emphasized': { value: '{colors.bg.emphasized}' },
        'bg.error': { value: '{colors.bg.error}' },
        'bg.warning': { value: '{colors.bg.warning}' },
        'bg.success': { value: '{colors.bg.success}' },

        'border.subtle': { value: '{colors.border.subtle}' },
        'border.emphasized': { value: '{colors.border.emphasized}' },
        'border.image': { value: '{colors.border.image}' },
        'border.error': { value: '{colors.border.error}' },
      },
    },
  },
});

/**
 * The Aurore Chakra system.
 */
export const system = createSystem(defaultConfig, config);
