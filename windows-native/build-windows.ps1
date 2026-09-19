$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
    New-Item -ItemType Directory -Force windows-native/artifacts | Out-Null
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    go build -o windows-native/artifacts/Misetanibox.Backend.exe ./windows-native/backend
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed' }
    go build -o windows-native/artifacts/MisetaniboxHelper.exe ./cmd/goclashz-helper
    if ($LASTEXITCODE -ne 0) { throw 'Helper build failed' }
    $project = 'windows-native/Misetanibox.Lite/Misetanibox.Lite.csproj'
    dotnet restore $project -r win-x64 -p:Platform=x64
    if ($LASTEXITCODE -ne 0) { throw 'WinUI restore failed' }
    dotnet publish $project --no-restore -c Release -r win-x64 -p:Platform=x64 --self-contained true -o windows-native/artifacts/package
    if ($LASTEXITCODE -ne 0) { throw 'WinUI publish failed' }
    # Keep the helper separate: compilation evidence, never installed or launched.
    New-Item -ItemType Directory -Force windows-native/artifacts/package/helper-not-enabled | Out-Null
    Copy-Item windows-native/artifacts/MisetaniboxHelper.exe windows-native/artifacts/package/helper-not-enabled/
    Copy-Item windows-native/artifacts/Misetanibox.Backend.exe windows-native/artifacts/package/
    Copy-Item windows-native/README.md windows-native/artifacts/package/
    foreach ($file in @('Misetanibox.Lite.exe', 'Misetanibox.Backend.exe', 'Microsoft.UI.Xaml.dll', 'coreclr.dll')) {
        if (!(Test-Path "windows-native/artifacts/package/$file")) { throw "Missing packaged dependency: $file" }
    }
    Compress-Archive -Path windows-native/artifacts/package/* -DestinationPath windows-native/artifacts/Misetanibox-Lite-Windows-x64-prototype.zip -Force
} finally { Pop-Location }
