# UPX 压缩构建产物（wails post-build hook 调用）。
# 用法: pwsh -NoProfile -File scripts\upx.ps1 <exe 路径>
# 未安装 upx 时跳过（不阻断构建）。
param([Parameter(Mandatory = $true)][string]$BinPath)

$upx = (Get-Command upx -ErrorAction SilentlyContinue).Source
if (-not $upx) {
    Write-Host "upx 未安装，跳过压缩（安装: winget install upx.upx 或 choco install upx）"
    exit 0
}
if (-not (Test-Path $BinPath)) {
    Write-Host "找不到构建产物 $BinPath，跳过压缩"
    exit 0
}

$before = (Get-Item $BinPath).Length
& $upx --best --force $BinPath | Out-Null
$after = (Get-Item $BinPath).Length

$beforeMB = [math]::Round($before / 1MB, 2)
$afterMB = [math]::Round($after / 1MB, 2)
$ratio = if ($before -gt 0) { [math]::Round($after * 100 / $before, 1) } else { 0 }
Write-Host "UPX: $beforeMB MB -> $afterMB MB ($ratio%)"
