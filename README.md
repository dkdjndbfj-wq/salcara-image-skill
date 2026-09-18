# Salcara Image Skill

一个面向 Codex 的生图与改图 Skill，通过 Salcara 的 OpenAI 兼容图片接口工作。安装后只需在本机完成一次配置，之后直接描述图片需求即可。

## 安装

最简单的方式，是在 Codex 中提供本仓库地址并说：

```text
请安装这个 Skill：https://github.com/dkdjndbfj-wq/salcara-image-skill
```

也可以下载 [最新 Release 压缩包](https://github.com/dkdjndbfj-wq/salcara-image-skill/releases/latest/download/salcara-image.zip)，解压后将整个目录放到：

- Windows：`%USERPROFILE%\.codex\skills\salcara-image`
- macOS / Linux：`~/.codex/skills/salcara-image`

最终应能看到 `.../salcara-image/SKILL.md`。

## 首次配置

不要把 API Key 粘贴到聊天里。请在本机终端运行配置向导，密钥输入过程不会显示。

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
- 不要将 API Key 提交到 GitHub、写入提示词或粘贴到聊天。
- 付费请求默认只尝试一次，避免网络状态不确定时重试导致重复生成与重复计费。

## 许可证

[MIT](LICENSE)
