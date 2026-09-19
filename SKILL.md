---
name: salcara-image
description: Generate or edit images through Salcara's OpenAI-compatible image API. Use when a user asks to create, transform, or batch-generate images with Salcara, Salcara.top, a Salcara API key, or the GPT Image models exposed by Salcara.
---

# Salcara Image

Use the bundled native CLI to call the user's OpenAI-compatible image endpoint. Never echo the API key or place it in command arguments or output.

## Required post-install handoff

Immediately after a successful installation, do not stop at a generic success message and do not start generating yet. If `configure --show` reports that no API key is configured, respond in the user's language with this four-item onboarding request. List every model and quality choice rather than asking an open-ended question:

> 已完成安装。请一次性回复下面 4 项，我会为你完成默认配置：
>
> 1. API 端点，例如 `https://salcara.top` 或带 `/v1` 的地址
> 2. API 密钥，例如 `sk-...`（仅在可信的私人对话中提供；也可以回复“本机输入”）
> 3. 默认模型：
>    - `gpt-image-2.5-sunburst`：质量和编辑精度优先
>    - `gpt-image-2.5-flare`：速度优先，适合日常生成
>    - `gpt-image-2.5`：兼容别名
>    - `gpt-image-2`：旧版兼容
>    - 或填写接口返回的其他模型 ID
> 4. 默认图片质量：`auto`、`low`、`medium`、`high`、`xhigh`、`max`
>
> 注意：`gpt-image-2` 只能选择 `auto`、`low`、`medium`、`high`。

Ask for all four items in one message. Do not silently select a model or quality. If the user explicitly says “使用推荐值” or leaves only those two choices to the agent, use `gpt-image-2.5-sunburst` and `max`. If the API returns a different model list, show that actual list and ask the user to choose again.

If `configure --show` reports an existing key, do not ask for it again. Confirm that installation is complete and report the saved endpoint, default model, and default quality without revealing the key.

## First-run setup

- When the user provides all four values in a trusted private conversation, pass the API key only through the CLI's standard input. Never place it in a command argument, print it, quote it back, or include it in the completion message. Run `configure` with the selected endpoint, model, and quality, then run `models` to verify the endpoint and ensure the chosen model is actually exposed.
- If the user replies “本机输入”, use the secure local wizard instead. On Windows, ask them to run `powershell -ExecutionPolicy Bypass -File scripts/setup-windows.ps1` from this skill directory. On macOS or Linux, ask them to run `sh scripts/setup-unix.sh`.
- The wizard asks for the API URL and key, fetches `/v1/models`, and lists model and quality choices. After setup succeeds, the skill is ready immediately; no Codex restart is needed.
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

1. Check configuration with `configure --show`. If no key is configured, perform the required post-install handoff above before any image request.
2. Clarify only choices that materially affect the requested result. Otherwise infer a reasonable prompt and make one image.
3. Do not ask the user to choose a model or quality on every request. Omit both `--model` and `--quality` so the CLI automatically uses the defaults selected during setup. Override either value only when the user explicitly asks or the requested capability requires it; a command-line override applies only to that request and does not change saved defaults.
4. For a batch or `high`/`xhigh`/`max` request, run `--dry-run` first and summarize the intended number of requests.
5. Generate or edit the image. Keep `--max-attempts 1` unless the user knowingly accepts possible duplicate billing.
6. Inspect every produced image with the local image-viewing tool. Check composition, text, transparency, and requested constraints. Iterate only when needed.
7. Return the absolute output paths and mention the model, size, and quality used.

## Conversational model and quality changes

Users can change the model or quality in ordinary conversation after setup; never require them to rerun onboarding or provide the endpoint/key again just for these changes.

- “这次用 `gpt-image-2.5-flare`，质量 `high`” means a one-request override. Pass `--model` and/or `--quality` to that generation or edit only; leave saved defaults unchanged.
- “以后默认用 `gpt-image-2.5-sunburst`，质量 `max`” or “记住这个设置” means a persistent change. Verify the model appears in `models`, then run `configure --default-model ... --default-quality ...` without changing the saved endpoint or key.
- If the user simply says “改成……” while requesting an image and does not say “默认”“以后” or “记住”, treat it as a one-request override and mention that the saved defaults were not changed.
- Enforce model compatibility: `gpt-image-2` accepts only `auto`, `low`, `medium`, and `high`; GPT Image 2.5 models may also use `xhigh` and `max`. If a combination is invalid, list the valid qualities and ask the user to choose.
- After a persistent change, confirm the new default model and quality without displaying the API key.

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
