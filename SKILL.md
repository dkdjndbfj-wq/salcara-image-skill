---
name: salcara-image
description: Generate or edit images through Salcara's OpenAI-compatible image API. Use when a user asks to create, transform, or batch-generate images with Salcara, Salcara.top, a Salcara API key, or the GPT Image models exposed by Salcara.
---

# Salcara Image

Use the bundled native CLI to call `https://salcara.top/v1` without exposing the user's key.

## First-run setup

- Never ask the user to paste an API key into chat. The bundled setup script prompts for it locally without displaying it.
- On Windows, ask the user to run `powershell -ExecutionPolicy Bypass -File scripts/setup-windows.ps1` from this skill directory.
- On macOS or Linux, ask the user to run `sh scripts/setup-unix.sh` from this skill directory.
- The wizard asks for the API URL, API key, default model, and default quality. It verifies `/v1/models` before saving the model and quality. If the user presses Enter without choosing, use `gpt-image-2.5-sunburst` and `max`. After setup succeeds, the skill is ready immediately; no Codex restart is needed.
- The default public URL is `https://salcara.top`. Salcara customers must use a Salcara-issued downstream key, never the operator's upstream key.
- The wizard stores the key in the current user's configuration directory with user-only permissions where the OS supports them. Environment variables `SALCARA_API_URL` and `SALCARA_API_KEY` may be used instead and take precedence.
- Prefer one attempt. Retrying an uncertain paid request can create and bill a duplicate image.

## Choose the binary

Use the binary matching the current OS and CPU:

- Windows x64: `bin/salcara-image-windows-amd64.exe`
- Windows ARM64: `bin/salcara-image-windows-arm64.exe`
- macOS Intel: `bin/salcara-image-darwin-amd64`
- macOS Apple Silicon: `bin/salcara-image-darwin-arm64`
- Linux x64: `bin/salcara-image-linux-amd64`
- Linux ARM64: `bin/salcara-image-linux-arm64`

Run the binary from this skill directory or use its absolute path. Read `references/models-and-parameters.md` when choosing a model, quality, size, format, or transparent background. Read `references/batch-format.md` for JSONL batch work.

## Workflow

1. Clarify only choices that materially affect the requested result. Otherwise infer a reasonable prompt and make one image.
2. Do not ask the user to choose a model or quality on every request. Omit both `--model` and `--quality` so the CLI automatically uses the defaults selected during setup. If setup choices were omitted, those defaults are `gpt-image-2.5-sunburst` and `max`. Override either value only when the user explicitly asks or the requested capability requires it; a command-line override applies only to that request and does not change saved defaults.
3. For a batch or `high`/`xhigh`/`max` request, run `--dry-run` first and summarize the intended number of requests.
4. Generate or edit the image. Keep `--max-attempts 1` unless the user knowingly accepts possible duplicate billing.
5. Inspect every produced image with the local image-viewing tool. Check composition, text, transparency, and requested constraints. Iterate only when needed.
6. Return the absolute output paths and mention the model, size, and quality used.

## Generate

```powershell
bin/salcara-image-windows-amd64.exe generate --prompt "A paper-cut mountain landscape at sunrise" --size 1024x1024 --out mountain.png
```

Use `--prompt-file` for long prompts. Exactly one of `--prompt` and `--prompt-file` is required.

Useful flags:

- `--n 1..10`
- `--background auto|opaque|transparent`
- `--output-format png|jpeg|webp`
- `--output-compression 0..100` for JPEG or WebP
- `--moderation auto|low`
- `--out PATH` or `--out-dir DIR`
- `--force` to replace an existing output intentionally
- `--dry-run` to validate without a network request

## Edit

```powershell
bin/salcara-image-windows-amd64.exe edit --image input.png --prompt "Keep the subject; replace the background with a quiet library" --out edited.png
```

Repeat `--image` to provide multiple reference images. Add `--mask mask.png` when a mask is available. Preserve the original unless the user explicitly asks to replace it.

## Batch

```powershell
bin/salcara-image-windows-amd64.exe generate-batch --file jobs.jsonl --out-dir generated --dry-run
bin/salcara-image-windows-amd64.exe generate-batch --file jobs.jsonl --out-dir generated
```

Batch execution is sequential by design. The command exits nonzero if any job fails and prints a JSON summary.

## Inspect models and diagnose

```powershell
bin/salcara-image-windows-amd64.exe models
```

If a request fails, report the sanitized API message and request ID. Do not reveal headers or environment values. A URL response and a base64 response are both valid; the CLI handles both.
