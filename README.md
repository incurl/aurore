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

| Feature | `image-01` / `image-01-live` | `MiniMax-H3-Image` |
| --- | --- | --- |
| Positive prompt | Yes (1,500 chars) | Yes (32,000 chars) |
| Negative prompt | **Folded into the positive prompt.** MiniMax has no `negative_prompt` field for either family. The UI labels this. | Same |
| Width / height / aspect ratio | Width and height, plus ratio, on `image-01` only | Ratio + area level (`1K`/`2K`/`4K`); no raw size |
| Batch size (`n`) | 1–9 | 1–10 |
| Seed | Yes (0 means the server picks one) | **None — the control is not rendered** |
| Reference image | Yes, one image, sent as a base64 data URL | Yes, 1–16, sent as data URIs to the edit endpoint |
| Mask / inpainting | **Applied locally.** MiniMax has no alpha mask field; the mask is composited onto the result. The UI labels this. | Same |
| Style preset | Yes, for `image-01-live` only | **None — the control is not rendered** |
| Watermark | Yes (`aigc_watermark`) | **None — the control is not rendered** |
| Prompt optimiser | Yes | Yes, sent inverted as `disable_prompt_enhanced` |
| Output format | Derived from the bytes | Same (`jpeg` by default) |
| CFG scale, sampler, steps, scheduler, LoRA, ControlNet | **Not supported — never rendered** | Same |

The Image-01 family is **synchronous**: one `POST /v1/image_generation`
returns the images inline. There is no task query endpoint. The job queue
exists to serialise requests against its documented **10 requests per
minute** limit, to allow cancellation, and to give the UI a durable
history.

### The H3 family is a different API

`MiniMax-H3-Image` and `MiniMax-H3-Image-Highspeed` are not extra model
ids on the endpoint above. They are served by two other paths:

| Mode | Endpoint |
| --- | --- |
| Text-to-image | `POST /v1/images/generations` |
| Image edit | `POST /v1/images/edits` |

The differences are not cosmetic, and the client models them rather than
papering over them:

- **Error envelope.** Image-01 always answers HTTP 200 and puts the code
  in `base_resp.status_code`. H3 uses an OpenAI-style body with a real
  HTTP status, and folds the numeric code into the message text
  (`"... (1004)"`). The client recovers the code, so the same codes and
  the same user-facing explanations work for both families.
- **Response shape.** Image-01 returns `data.image_base64[]` and
  `metadata.success_count` (as a string, see below). H3 returns
  `data[]` where each entry is an image **or** an error, plus
  `usage.generated_images` and a `request_id`.
- **Rate limit.** Image-01 is metered at 10 requests per minute. H3 is
  metered at **1000 images per minute**, so a batch of ten spends ten of
  them. The two limits are separate token buckets.
- **Timeout.** The H3 server waits up to 600 seconds and asks clients to
  allow at least that long. The shared HTTP client is therefore bounded
  at the H3 ceiling, and each family applies its own tighter context
  timeout. A short H3 timeout would abandon a request that is still
  being generated and is still billed.
- **Output size.** H3 rejects an out-of-range `size` instead of scaling
  it, and its minimum area is 655,360 pixels — larger than `512x512`.
  Aurore therefore sends an aspect ratio and an area level, which cannot
  fall outside the limits, and only sends an explicit `size` when the
  pair is valid.

The H3 API is **not idempotent**: once a request is sent, the server
finishes and charges for the successful images even if the client
disconnects, and a retry generates and charges again. Aurore does not
retry a generation automatically for this reason.

One quirk is worth recording, because the published schema and the live
API disagree: `metadata.success_count` and `metadata.failed_count` are
documented as integers, but MiniMax sends them as JSON **strings**
(`"success_count": "3"`). The client accepts both forms, so the strict
integer reading must not be reintroduced. A body this client cannot read
is reported with Aurore's own code `-1`, never with MiniMax's `2013`,
which means the request parameters were rejected.

MiniMax returns **JPEG** images. The format is detected from the image
bytes when an image is stored, so the stored file carries the extension
it really has (`image_1_xxx.jpg`, not `.png`), and the export button
names the downloaded file after that format too.

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
