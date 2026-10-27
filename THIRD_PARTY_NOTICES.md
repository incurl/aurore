# Third party notices

Aurore is built on the following third party projects. Every licence
below is permissive; **no GPL, AGPL or other copyleft dependency is
present in the dependency tree**. This is verified automatically by
`tools/license-audit.py`, which exits non-zero if a copyleft licence is
introduced.

## InvokeAI

Some frontend components are derived from the InvokeAI web frontend.

- **Project**: InvokeAI
- **Source**: https://github.com/invoke-ai/InvokeAI
- **Files used**: `invokeai/frontend/webv2/src/**`
- **Licence**: Apache License 2.0
- **Copyright**: Copyright 2022-2026 InvokeAI Contributors

Each vendored file in `frontend/src/wrapped-invoke-ui/` retains the
Apache-2.0 header and records the upstream path it was adapted from.

InvokeAI is used here **purely as a UI component library**. None of its
native diffusion backend, model runtime, checkpoint manager, node
workflow engine or GPU inference code is included, bundled, imported or
invoked by Aurore. All image generation requests are sent to the remote
MiniMax Image-01 API.

### What was adapted

The upstream components are coupled to InvokeAI's own runtime. The
adaptation replaces those couplings with local equivalents:

| Upstream | Aurore |
| --- | --- |
| `@platform/*`, `@features/*`, `@theme/*` module specifiers | `@wrapped/*`, resolved to `frontend/src/wrapped-invoke-ui/` |
| Widget overlay registry (`./widgetOverlays`) | Dropped: Aurore has no floating widgets |
| Menu open-menu bookkeeping (`@platform/state/externalStore`) | Dropped: Aurore has no floating widgets |
| Chakra tooltip with the widget overlay hook | Simplified to a plain Chakra v3 tooltip |
| Wildcard library (`@features/generation/core/dynamicPrompts`) | Kept, but no wildcard is ever known: Aurore has no wildcard library |
| InvokeAI's Chakra theme | Aurore's own theme with the same semantic token names |

## Go dependencies

| Project | Licence |
| --- | --- |
| [Wails](https://github.com/wailsapp/wails) v3 | MIT |
| [PocketBase](https://github.com/pocketbase/pocketbase) v0.39.11 | MIT |

## npm dependencies

| Package | Licence |
| --- | --- |
| `@chakra-ui/react` | MIT |
| `@emotion/react` | MIT |
| `@tanstack/react-query` | MIT |
| `@wailsio/runtime` | MIT |
| `i18next` | MIT |
| `lucide-react` | ISC |
| `react`, `react-dom` | MIT |
| `react-i18next` | MIT |
| `typescript` | Apache-2.0 |
| `vite`, `vitest`, `@vitejs/plugin-react` | MIT |

## Verifying

```bash
python3 tools/license-audit.py
```

The command prints the licence breakdown and fails if a copyleft
licence appears.
