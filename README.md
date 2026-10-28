# Aurore

A self-contained desktop image generation application.

The UI shell is derived from the [InvokeAI](https://github.com/invoke-ai/InvokeAI)
web frontend (Apache-2.0), used purely as a component library. **None of
InvokeAI's native diffusion backend, model runtime, checkpoint manager or
node workflow engine is included or invoked.** Every generation request
goes to the remote **MiniMax Image-01** API.

Persistence, file storage, migrations, access rules and record lifecycle
hooks are provided by **PocketBase, embedded as a Go library**. PocketBase
is never run as a standalone process: its REST and file API is mounted on
the Wails asset server, so images are served from the UI's own origin
with no second port.

## Architecture

```
Wails webview
├── React 19 + Vite + TypeScript + TanStack Query
│   ├── wrapped-invoke-ui/   vendored InvokeAI components (Apache-2.0)
│   ├── invoke-adapter/      UI payload → API-shaped request (pure)
│   └── wails-bindings/      typed wrappers over the generated bindings
│
└──── Wails bindings (primary channel) ───────────────────────┐
        Go: bindings/ → jobs/ → minimax/  (MiniMax HTTPS API)  │
                            └→ pocketbase/ (embedded, library) │
                                     └→ migrations, file fields │
        PocketBase's REST + file API mounted at /api/* on the    │
        same origin as the UI.                                   │
```

## Requirements

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.26+ | `toolchain go1.26.10` is pinned in `go.mod` |
| Node | 20+ | |
| Wails CLI | v3.0.0 | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0` |
| Linux build deps | gtk3, webkit2gtk-4.1, gcc | `pkg-config` must find them |

> The Wails v3 CLI binary is **`wails3`**, not `wails`, and the project
> configuration is **`build/config.yml`**, not `wails.json`.

## Getting started

```bash
# 1. Install the Wails v3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

# 2. Install the frontend dependencies
cd frontend && npm install && cd ..

# 3. Generate the TypeScript bindings
wails3 generate bindings -ts -d ./frontend/bindings

# 4. Run the app in development mode
wails3 dev
```

`wails3 dev` starts the Vite dev server, regenerates the bindings and
rebuilds the Go backend on every change.

To build a distributable binary:

```bash
wails3 build      # → build/bin/aurore
```

## Using the app

1. Open **Settings**, paste your MiniMax API key and press **Test
   connection**. The key is stored encrypted in the local database; it is
   never returned to the frontend.
2. Write a positive prompt (and optionally a negative prompt).
3. Optionally upload a reference image, or paint a mask over the
   preview.
4. Adjust the supported parameters and press **Generate**.
5. The job appears in the queue with a live status. Completed images
   appear in the gallery and can be previewed, compared and exported.

### What MiniMax Image-01 actually supports

This is enforced by a capability gate, not by hiding controls with CSS.
The adapter and the UI never model an unsupported concept, and
`compliance.test.ts` fails the build if one appears.

| Feature | Support |
| --- | --- |
| Positive prompt | Yes |
| Negative prompt | **Folded into the positive prompt.** MiniMax has no `negative_prompt` field. The UI labels this. |
| Width / height / aspect ratio | Yes, for `image-01` only |
| Batch size (`n`, 1–9) | Yes |
| Seed | Yes (0 means the server picks one) |
| Reference image | Yes, one image, sent as a base64 data URL |
| Mask / inpainting | **Applied locally.** MiniMax has no alpha mask field; the mask is composited onto the result. The UI labels this. |
| Style preset | Yes, for `image-01-live` only |
| Watermark | Yes (`aigc_watermark`) |
| Prompt optimiser | Yes |
| CFG scale, sampler, steps, scheduler, LoRA, ControlNet | **Not supported — never rendered** |

The MiniMax image generation API is **synchronous**: one `POST
/v1/image_generation` returns the images inline. There is no task query
endpoint. The job queue exists to serialise requests against the
documented **10 requests per minute** limit, to allow cancellation, and
to give the UI a durable history.

One quirk is worth recording, because the published schema and the live
API disagree: `metadata.success_count` and `metadata.failed_count` are
documented as integers, but MiniMax sends them as JSON **strings**
(`"success_count": "3"`). The client accepts both forms, so the strict
integer reading must not be reintroduced. A body this client cannot read
is reported with Aurore's own code `-1`, never with MiniMax's `2013`,
which means the request parameters were rejected.

## Data

Everything lives in `pb_data/` next to the executable:

| Path | Contents |
| --- | --- |
| `pb_data/data.db` | The SQLite database (5 collections) |
| `pb_data/storage/` | Uploaded and generated files, served by PocketBase's native file route |
| `pb_data/.aurore_key` | The AES-256-GCM key for the API key (mode 0600) |

Set `AURORE_DATA_DIR` to relocate it.

## Collections

| Collection | Purpose |
| --- | --- |
| `image_jobs` | Job metadata, parameters, status, MiniMax task id, errors |
| `generated_images` | The generated artwork, file attachment, thumbnail, seed |
| `prompt_presets` | Saved prompt templates and their default parameters |
| `app_settings` | The encrypted API key and the default preferences |
| `uploaded_assets` | Uploaded reference images and masks |

All writes go through the Go services: the `create`, `update` and `delete`
rules are closed, so the frontend can only read. PocketBase's own rules,
migrations, file fields and hooks are used as-is.

## Tests

```bash
go test ./...                 # MiniMax client, mapping, rate limiter,
                               # PocketBase schema and encryption,
                               # job lifecycle, asset routing
cd frontend && npm test        # adapter mapping and compliance
python3 tools/license-audit.py # no GPL/AGPL dependency
```

## Licence

Aurore is MIT licensed. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
for the third party attributions.
