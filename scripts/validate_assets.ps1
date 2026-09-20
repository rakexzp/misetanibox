param(
    [string]$AssetRoot = ".\build\runtime-assets"
)

$ErrorActionPreference = "Stop"
$ErrorCount = 0
# Accept both the bundle root and the existing release invocation's core/bin.
$AssetRoot = (Resolve-Path $AssetRoot).Path
if ((Split-Path $AssetRoot -Leaf) -eq "bin" -and (Split-Path (Split-Path $AssetRoot -Parent) -Leaf) -eq "core") {
    $AssetRoot = Split-Path (Split-Path $AssetRoot -Parent) -Parent
}
$binRoot = Join-Path $AssetRoot "core\bin"

function Assert-FileExists {
    param([string]$Path, [string]$Label)
    if (!(Test-Path $Path)) {
        Write-Host "FAIL: $Label 不存在: $Path" -ForegroundColor Red
        $script:ErrorCount++
        return $false
    }
    return $true
}

function Assert-MinSize {
    param([string]$Path, [string]$Label, [int64]$MinBytes)
    $item = Get-Item $Path
    if ($item.Length -lt $MinBytes) {
        Write-Host "FAIL: $Label 体积过小: $($item.Length) bytes (最小预期 $MinBytes bytes)" -ForegroundColor Red
        $script:ErrorCount++
        return $false
    }
    return $true
}

function Assert-MZHeader {
    param([string]$Path, [string]$Label)
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    if ($bytes.Length -lt 2 -or $bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) {
        Write-Host "FAIL: $Label 没有有效的 MZ 头 (不是 Windows PE 文件)" -ForegroundColor Red
        $script:ErrorCount++
        return $false
    }
    return $true
}

function Assert-NotHTML {
    param([string]$Path, [string]$Label)
    $header = Get-Content -Path $Path -TotalCount 1 -ErrorAction SilentlyContinue
    if ($header -and ($header -match '<html|<!doctype html|<head')) {
        Write-Host "FAIL: $Label 内容像 HTML 错误页，不是有效的数据文件" -ForegroundColor Red
        $script:ErrorCount++
        return $false
    }
    return $true
}

Write-Host "===== Misetanibox: проверка сборочных ассетов =====" -ForegroundColor Cyan
Write-Host "资产目录: $AssetRoot"
Write-Host ""

# === 1. 二进制可执行文件校验 ===

# 主程序
if (Assert-FileExists ".\build\bin\Misetanibox.exe" "主程序") {
    Assert-MZHeader ".\build\bin\Misetanibox.exe" "主程序" | Out-Null
}

# Helper
if (Assert-FileExists ".\build\bin\MisetaniboxHelper.exe" "Helper 服务") {
    Assert-MZHeader ".\build\bin\MisetaniboxHelper.exe" "Helper 服务" | Out-Null
}

# === 2. Mihomo 内核校验 ===
$clashPath = "$binRoot\clash.exe"
if (Assert-FileExists $clashPath "Mihomo") {
    Assert-MZHeader $clashPath "Mihomo" | Out-Null
    Assert-MinSize $clashPath "Mihomo" (5 * 1024 * 1024) | Out-Null
    $clashPath = (Resolve-Path $clashPath).Path
    $probeDir = Join-Path $env:TEMP ("mihomo-probe-" + [guid]::NewGuid())
    New-Item -ItemType Directory $probeDir | Out-Null
    try {
        foreach ($stack in @("gvisor", "mips", "misetanibox-invalid-stack")) {
            $yaml = "mode: direct`nlog-level: silent`ndns:`n  enable: false`ntun:`n  enable: false`n  stack: $stack`nrules: []`n"
            $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($yaml))
            $info = New-Object System.Diagnostics.ProcessStartInfo
            $info.FileName = $clashPath
            $info.Arguments = "-t -config $encoded -d `"$probeDir`""
            $info.WorkingDirectory = $probeDir
            $info.UseShellExecute = $false
            $info.CreateNoWindow = $true
            foreach ($key in @($info.EnvironmentVariables.Keys)) {
                if ($key -match '^(CLASH_|MIHOMO_)') { $info.EnvironmentVariables.Remove($key) }
            }
            $process = [System.Diagnostics.Process]::Start($info)
            if (!$process.WaitForExit(10000)) { $process.Kill(); throw "Mihomo probe timed out: $stack" }
            $code = $process.ExitCode
            $process.Dispose()
            if (($stack -eq "misetanibox-invalid-stack" -and $code -eq 0) -or ($stack -ne "misetanibox-invalid-stack" -and $code -ne 0)) {
                throw "Mihomo parser control failed: $stack (exit $code)"
            }
        }
        Write-Host "OK: gVisor/MIPS parser probes; this is NOT a working-TUN test"
    } finally { Remove-Item $probeDir -Recurse -Force }
}

# === 3. Wintun 驱动 DLL 校验 ===
$wintunPath = "$binRoot\wintun.dll"
if (Assert-FileExists $wintunPath "Wintun DLL") {
    Assert-MZHeader $wintunPath "Wintun DLL" | Out-Null
    Assert-MinSize $wintunPath "Wintun DLL" (32 * 1024) | Out-Null
}

# === 4. Geo 数据库文件校验 ===
$geoFiles = @(
    @{ Name = "geoip.metadb"; Label = "GeoIP"; MinSize = 64 * 1024 },
    @{ Name = "geosite.dat"; Label = "GeoSite"; MinSize = 64 * 1024 },
    @{ Name = "country.mmdb"; Label = "MMDB"; MinSize = 64 * 1024 },
    @{ Name = "asn.dat"; Label = "ASN"; MinSize = 64 * 1024 }
)

foreach ($geo in $geoFiles) {
    $geoPath = "$binRoot\$($geo.Name)"
    if (Assert-FileExists $geoPath $geo.Label) {
        Assert-MinSize $geoPath $geo.Label $geo.MinSize | Out-Null
        Assert-NotHTML $geoPath $geo.Label | Out-Null
    }
}

# === 5. Manifest SHA256 校验 ===
$manifestPath = "$AssetRoot\core\asset-manifest.json"
$manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
foreach ($name in @("clash.exe", "wintun.dll", "geoip.metadb", "geosite.dat", "country.mmdb", "asn.dat")) {
    $entries = @($manifest.assets | Where-Object { $_.name -eq $name })
    if ($entries.Count -ne 1) { throw "Missing/duplicate manifest entry: $name" }
    $asset = $entries[0]
    if ($asset.path -ne "core/bin/$name") { throw "Invalid manifest path: $name" }
    $assetPath = Join-Path $AssetRoot $asset.path
    if ($asset.sha256 -notmatch '^[a-fA-F0-9]{64}$') { throw "Missing/invalid SHA256: $name" }
    $actualHash = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLower()
    if ($actualHash -ne $asset.sha256.ToLower()) { throw "SHA256 mismatch: $name" }
    if ($name -eq "clash.exe" -and $asset.version -ne "v1.19.31") { throw "Unexpected stock version" }
}

# === 结果汇总 ===
Write-Host ""
if ($ErrorCount -gt 0) {
    Write-Host "===== 校验失败: $ErrorCount 个错误 =====" -ForegroundColor Red
    exit 1
} else {
    Write-Host "===== 全部校验通过 =====" -ForegroundColor Green
    exit 0
}
