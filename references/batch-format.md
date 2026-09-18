# Batch JSONL format

Write one JSON object per line. Blank lines and lines beginning with `#` are ignored.

```jsonl
{"prompt":"A blue ceramic mug on white","out":"mug.png"}
{"prompt":"A red paper kite in a clear sky","size":"1536x1024","quality":"medium","n":2,"out":"kite.png"}
```

Each job requires `prompt`. Supported optional fields are:

- `model`
- `size`
- `quality`
- `n`
- `background`
- `output_format`
- `output_compression`
- `moderation`
- `out`

Command-line defaults apply when a field is omitted. Relative `out` paths resolve under `--out-dir`. With multiple images, the CLI inserts `-1`, `-2`, and so on before the extension.

Batch jobs run sequentially to control spend and rate pressure. Use unique output names and start with `--dry-run`.

