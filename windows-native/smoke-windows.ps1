$ErrorActionPreference = 'Stop'
# Run only the extracted distributable; never install the helper or click Connect.
$work = Join-Path $env:RUNNER_TEMP ('native-smoke-' + [guid]::NewGuid().ToString('N'))
$report = Join-Path $PSScriptRoot 'artifacts/smoke'
New-Item -ItemType Directory -Force $work, $report | Out-Null
Expand-Archive (Join-Path $PSScriptRoot 'artifacts/Misetanibox-Lite-Windows-x64-prototype.zip') (Join-Path $work 'package')
$interactive = [Environment]::UserInteractive -and [Diagnostics.Process]::GetCurrentProcess().SessionId -ne 0
$start = Get-Date
$process = $null
try {
    $info = [Diagnostics.ProcessStartInfo]::new((Join-Path $work 'package/Misetanibox.Lite.exe'))
    $info.UseShellExecute = $false
    $info.WorkingDirectory = Join-Path $work 'package'
    $info.Environment['LOCALAPPDATA'] = Join-Path $work 'local'
    $info.Environment['GOCLASHZ_DATA_DIR'] = Join-Path $work 'data'
    New-Item -ItemType Directory -Force $info.Environment['LOCALAPPDATA'], $info.Environment['GOCLASHZ_DATA_DIR'] | Out-Null
    $process = [Diagnostics.Process]::Start($info)
    # KnownFolder APIs may ignore LOCALAPPDATA; only collect this fresh process's lines.
    $logs = @((Join-Path $info.Environment['LOCALAPPDATA'] 'Misetanibox.Lite/logs/startup.log'), (Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Misetanibox.Lite/logs/startup.log')) | Select-Object -Unique
    $deadline = (Get-Date).AddSeconds(60)
    $stages = @()
    do {
        Start-Sleep -Milliseconds 500
        $stages = @($logs | Where-Object { Test-Path $_ } | ForEach-Object { Get-Content $_ } | Where-Object { $_ -match "pid=$($process.Id) " })
        $loaded = [bool]($stages -match ' window.loaded$')
        $ready = [bool]($stages -match ' backend.ready$')
        if ($process.HasExited -or ($loaded -and $ready)) { break }
    } while ((Get-Date) -lt $deadline)
    # Logs contain fixed stages, exception types/HRESULTs and method names, not messages.
    foreach ($log in $logs) {
        if ((Test-Path $log) -and (Get-Item $log).LastWriteTime -ge $start) {
            Get-Content $log | Add-Content (Join-Path $report 'startup.log')
        }
    }
    $result = [ordered]@{ interactive = $interactive; pid = $process.Id; exited = $process.HasExited; exitCode = $null; windowLoaded = $loaded; backendReady = $ready; mainWindowHandle = 0; outcome = 'inconclusive'; limitation = 'Startup stages only; no visual, input, VPN or helper validation.' }
    if ($process.HasExited) { $result.exitCode = $process.ExitCode }
    else { $process.Refresh(); $result.mainWindowHandle = $process.MainWindowHandle.ToInt64() }
    if (!$process.HasExited -and $loaded -and $ready -and $result.mainWindowHandle -ne 0) { $result.outcome = 'startup-stages-passed' }
    elseif ($interactive -or $process.HasExited) { $result.outcome = 'failed' }
    $result | ConvertTo-Json | Set-Content (Join-Path $report 'result.json')
    $result | ConvertTo-Json | Write-Output
    if ($result.outcome -ne 'startup-stages-passed') {
        # Avoid Event.Message, which may contain arbitrary command lines or user data.
        Get-WinEvent -FilterHashtable @{ LogName = 'Application'; StartTime = $start; Level = 2 } -ErrorAction SilentlyContinue |
            Select-Object TimeCreated, ProviderName, Id, ProcessId | ConvertTo-Json | Set-Content (Join-Path $report 'event-metadata.json')
        if ($result.outcome -eq 'failed') { throw 'Extracted ZIP startup failed; inspect smoke artifact.' }
        Write-Warning 'GUI unavailable: startup validation is inconclusive, not passed.'
    }
} finally {
    if ($null -ne $process) {
        if (!$process.HasExited) {
            # Close may minimize to tray; force only the exact process we launched.
            $null = $process.CloseMainWindow()
            if (!$process.WaitForExit(5000)) { $process.Kill(); $process.WaitForExit() }
        }
        $process.Dispose()
    }
}
