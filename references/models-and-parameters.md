# Models and parameters

## Model selection

| Model | Prefer it for |
|---|---|
| `gpt-image-2.5-sunburst` | Best fidelity, detailed final images, precise edits; setup default |
| `gpt-image-2.5-flare` | Faster drafts and normal generation |
| `gpt-image-2.5` | Compatibility alias only; use when the deployment documents its routing |
| `gpt-image-2` | Legacy compatibility |

When no model or quality override is provided, the CLI uses the saved setup choices. A fresh or legacy configuration falls back to `gpt-image-2.5-sunburst` and `max`.

## Quality

- `low`: fast drafts.
- `medium`: balanced output.
- `high`: detailed finals.
- `xhigh` and `max`: GPT Image 2.5 only; `max` is the setup default for GPT Image 2.5 models.
- `auto`: lets the model select.

## Size

Supported common values are `1024x1024`, `1536x1024`, `1024x1536`, and `auto`. GPT Image 2/2.5 may also accept arbitrary dimensions when both sides are divisible by 16, the aspect ratio is at most 3:1, total pixels are between 655,360 and 8,294,400, and the longest edge is at most 3840.

## Format and background

- Use PNG for transparent backgrounds or lossless assets.
- Use JPEG for photographs where small files matter.
- Use WebP for compact web assets.
- `--output-compression` applies only to JPEG and WebP.
- For transparency, set `--background transparent` and use PNG or WebP.

## Request safety

- Use `--max-attempts 3`: one initial attempt plus at most two retries while no usable image has been received. Stop after three total attempts, on a non-retryable error, or as soon as an image item is received.
- Use `--dry-run` before a large batch.
