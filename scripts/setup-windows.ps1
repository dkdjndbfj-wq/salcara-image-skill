$ErrorActionPreference = "Stop"

$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$binaryName = if ($architecture -eq "arm64") { "salcara-image-windows-arm64.exe" } else { "salcara-image-windows-amd64.exe" }
$binary = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\bin\$binaryName"))

if (-not (Test-Path -LiteralPath $binary)) {
    throw "找不到客户端：$binary"
}

$apiUrl = Read-Host "Salcara API URL（直接回车使用 https://salcara.top）"
if ([string]::IsNullOrWhiteSpace($apiUrl)) {
    $apiUrl = "https://salcara.top"
}

$secureKey = Read-Host "Salcara API Key（输入不会显示）" -AsSecureString
$keyPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureKey)
try {
    $plainKey = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($keyPointer)
    $plainKey | & $binary configure --api-url $apiUrl --key-stdin --default-model gpt-image-2.5-sunburst --default-quality max
    if ($LASTEXITCODE -ne 0) { throw "保存配置失败" }
} finally {
    if ($null -ne $plainKey) { $plainKey = $null }
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($keyPointer)
}

$modelResponse = & $binary models | Out-String
if ($LASTEXITCODE -ne 0) { throw "URL 或 Key 验证失败，请检查后重试" }
$modelJson = $modelResponse | ConvertFrom-Json
$available = @($modelJson.data | ForEach-Object { $_.id } | Where-Object { $_ -like "gpt-image*" })
if ($available.Count -eq 0) { throw "接口未返回可用的 GPT Image 模型" }

Write-Host ""
Write-Host "请选择默认模型："
for ($i = 0; $i -lt $available.Count; $i++) {
    $description = switch ($available[$i]) {
        "gpt-image-2.5-flare" { "速度快，适合日常和草稿" }
        "gpt-image-2.5-sunburst" { "质量和编辑精度优先（默认）" }
        "gpt-image-2" { "旧版兼容" }
        default { "兼容模型" }
    }
    Write-Host "$($i + 1). $($available[$i]) — $description"
}
$choice = Read-Host "输入序号（直接回车优先选择 Sunburst）"
$selected = $null
if ([string]::IsNullOrWhiteSpace($choice)) {
    $selected = $available | Where-Object { $_ -eq "gpt-image-2.5-sunburst" } | Select-Object -First 1
    if ($null -eq $selected) { $selected = $available[0] }
} elseif ($choice -match '^\d+$' -and [int]$choice -ge 1 -and [int]$choice -le $available.Count) {
    $selected = $available[[int]$choice - 1]
} else {
    throw "模型序号无效"
}

$qualities = if ($selected -eq "gpt-image-2") {
    @("high", "medium", "low", "auto")
} else {
    @("max", "xhigh", "high", "medium", "low", "auto")
}

Write-Host ""
Write-Host "请选择默认画质："
for ($i = 0; $i -lt $qualities.Count; $i++) {
    $suffix = if ($i -eq 0) { "（默认）" } else { "" }
    Write-Host "$($i + 1). $($qualities[$i])$suffix"
}
$qualityChoice = Read-Host "输入序号（直接回车使用 $($qualities[0])）"
if ([string]::IsNullOrWhiteSpace($qualityChoice)) {
    $selectedQuality = $qualities[0]
} elseif ($qualityChoice -match '^\d+$' -and [int]$qualityChoice -ge 1 -and [int]$qualityChoice -le $qualities.Count) {
    $selectedQuality = $qualities[[int]$qualityChoice - 1]
} else {
    throw "画质序号无效"
}

& $binary configure --api-url $apiUrl --default-model $selected --default-quality $selectedQuality
if ($LASTEXITCODE -ne 0) { throw "保存默认模型或画质失败" }
Write-Host ""
Write-Host "配置完成。默认模型：$selected；默认画质：$selectedQuality"
