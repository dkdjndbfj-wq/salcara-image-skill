# Salcara Image Skill

一个面向 Codex 的生图与改图 Skill，通过 Salcara 的 OpenAI 兼容图片接口工作。安装后只需在本机完成一次配置，之后直接描述图片需求即可。

## 安装

最简单的方式，是在 Codex 中提供本仓库地址并说：

```text
请安装这个 Skill：https://github.com/dkdjndbfj-wq/salcara-image-skill
```

安装完成后，Agent 应当直接回复“已完成安装”，然后一次性询问 API 端点、API 密钥、默认模型和默认图片质量。默认模型和质量不是开放式填写，Agent 会列出下面的完整选项供选择。

也可以下载 [最新 Release 压缩包](https://github.com/dkdjndbfj-wq/salcara-image-skill/releases/latest/download/salcara-image.zip)，解压后将整个目录放到：

- Windows：`%USERPROFILE%\.codex\skills\salcara-image`
- macOS / Linux：`~/.codex/skills/salcara-image`

最终应能看到 `.../salcara-image/SKILL.md`。

## 首次配置

Agent 会一次性询问以下四项：

1. API 端点，例如 `https://salcara.top` 或带 `/v1` 的地址。
2. API 密钥。只应在可信的私人对话中提供；如果不想粘贴到聊天里，可以回复“本机输入”。
3. 默认模型：
   - `gpt-image-2.5-sunburst`：质量和编辑精度优先
   - `gpt-image-2.5-flare`：速度优先，适合日常生成
   - `gpt-image-2.5`：兼容别名
   - `gpt-image-2`：旧版兼容
   - 或接口实际返回的其他模型 ID
4. 默认图片质量：`auto`、`low`、`medium`、`high`、`xhigh`、`max`。

`gpt-image-2` 只支持 `auto`、`low`、`medium`、`high`。选择完成后，Agent 会验证 `/v1/models` 并保存配置，不会在回复中重复显示密钥。明确回复“使用推荐值”时，模型和质量分别采用 `gpt-image-2.5-sunburst` 与 `max`。

如果选择“本机输入”，请在本机终端运行配置向导，密钥输入过程不会显示。

Windows PowerShell：

```powershell
cd "$HOME\.codex\skills\salcara-image"
powershell -ExecutionPolicy Bypass -File .\scripts\setup-windows.ps1
```

macOS / Linux：

```bash
cd ~/.codex/skills/salcara-image
sh scripts/setup-unix.sh
```

向导会询问 API URL、API Key、默认模型和默认画质。直接回车时采用：

- API URL：`https://salcara.top`
- 默认模型：`gpt-image-2.5-sunburst`
- 默认画质：`max`

向导会先读取接口实际返回的模型，再保存配置。配置仅保存在当前用户目录，不会写进 Skill，也不会上传到聊天。

## 使用示例

配置完成后，可以直接在 Codex 中说：

```text
使用 $salcara-image 生成一张奥特曼打怪兽的图片。
```

默认会使用首次配置时保存的模型与画质。只有当你明确提出时，才会对当前请求临时切换，例如：

```text
使用 $salcara-image 生成一张雨夜霓虹街道，横版，这次改用 low 画质预览，不要修改默认设置。
```

模型和画质也可以在后续对话中直接修改：

```text
这次改用 gpt-image-2.5-flare 和 high，只修改当前请求。
```

```text
以后默认使用 gpt-image-2.5-sunburst 和 max，记住这个设置。
```

包含“这次”“当前”的指令只影响一次生成；包含“以后默认”“记住”的指令会更新保存的默认值，不需要重新提供 API 端点或密钥。

## 每次生成前内部检查

生图、改图、重做、继续优化和失败后的重试都会产生新的付费请求。用户提出生成或编辑图片后，Agent 会在内部检查最终提示词、操作类型、参考图、模型、画质、尺寸、背景、格式和数量，不会再弹出一次确认步骤打断用户。

没有明确数量时固定生成 1 张；只有用户明确要求时才会多图或批量生成。参数缺失但不影响核心结果时，Agent 会选择稳妥值直接执行；只有当缺失信息会明显改变图片或付费数量时才会询问。如果没有收到可用图片，允许初次请求后自动重试最多 2 次，也就是最多尝试 3 次；收到图片、遇到不可重试错误或三次仍无图片时立即暂停，绝不会自动发起第 4 次。不会因为图片效果不理想而擅自重做。

## 大量文字的图片

制作海报、菜单、价格表、信息图、活动日程或其他包含大量准确文字的图片时，不会让生图模型直接绘制这些文字。Agent 会先规划标题、正文、价格、日期等区域的位置、层级、留白和安全边距，然后只生成没有文字、没有伪文字并预留干净区域的底图。

底图生成后，Agent 再使用 SVG、HTML/Canvas、Sharp、ImageMagick 等本地确定性排版方式，把用户提供的准确文字叠加进去，并检查错别字、换行、对齐、对比度和裁切。后续本地文字排版不调用 Salcara 生图接口，不会产生第二次生图费用；能通过调整排版解决的问题不会重新生成底图。

编辑已有图片：

```text
使用 $salcara-image，把这张图片的背景改成安静的图书馆，保持人物不变。
```

## 支持的平台

仓库内包含以下预编译客户端：

- Windows x64 / ARM64
- macOS Intel / Apple Silicon
- Linux x64 / ARM64

客户端源代码位于 `src/main.go`，二进制校验值位于 `SHA256SUMS`。

## 安全说明

- 只使用你自己的 Salcara 下游 API Key。
- 不要将 API Key 提交到 GitHub 或写进公开提示词；只在可信的私人对话中提供，或选择“本机输入”。
- 没有收到图片时最多自动重试 2 次；三次尝试仍无图片即暂停并报告错误。

## 许可证

[MIT](LICENSE)
