param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$previewRepo = Split-Path $PSScriptRoot -Parent
$previewDir = Join-Path $previewRepo '.dev/media-preview'
New-Item -ItemType Directory -Force -Path $previewDir | Out-Null
$mainFile = Join-Path $previewDir 'main.json'
function New-PreviewSecret {
  $secretBytes = New-Object byte[] 32
  [System.Security.Cryptography.RandomNumberGenerator]::Fill($secretBytes)
  [Convert]::ToBase64String($secretBytes)
}
if (!(Test-Path -LiteralPath $mainFile)) {
  @{
    enabled = $false; mediad_base_url = 'http://127.0.0.1:18090'
    management_token = New-PreviewSecret; relay_secret = New-PreviewSecret; callback_secret = New-PreviewSecret
    bindings_path = (Join-Path $previewDir 'bindings.json'); forward_timeout_seconds = 900; management_timeout_seconds = 30
  } | ConvertTo-Json | Set-Content -LiteralPath $mainFile -Encoding utf8NoBOM
}
$previewSettings = Get-Content -LiteralPath $mainFile -Raw | ConvertFrom-Json
# Only stop processes started by this script, after checking their exact executable/command line.
$pidFile = Join-Path $previewDir 'processes.json'
if (Test-Path -LiteralPath $pidFile) {
  foreach ($saved in (Get-Content -LiteralPath $pidFile -Raw | ConvertFrom-Json)) {
    $existing = Get-CimInstance Win32_Process -Filter "ProcessId = $($saved.id)"
    if ($existing -and $existing.ExecutablePath -eq $saved.path -and $existing.CreationDate.ToUniversalTime().Ticks -eq ([datetime]$saved.created).ToUniversalTime().Ticks) { Stop-Process -Id $saved.id }
  }
}
if (!$SkipBuild) {
  Push-Location (Join-Path $previewRepo 'mediad')
  try { & go build -o (Join-Path $previewDir 'mediad.exe') ./cmd/mediad; if ($LASTEXITCODE) { throw 'mediad build failed' } } finally { Pop-Location }
  Push-Location (Join-Path $previewRepo 'backend')
  try { & go build -o (Join-Path $previewDir 'admin-preview.exe') ./cmd/mediaservicedev; if ($LASTEXITCODE) { throw 'admin preview build failed' } } finally { Pop-Location }
}
foreach ($previewPort in @(18090, 18131, 3000)) {
  if (Get-NetTCPConnection -State Listen -LocalPort $previewPort -ErrorAction SilentlyContinue) { throw "Port $previewPort is busy; no unrelated service was stopped." }
}
$env:MEDIAD_LISTEN = '127.0.0.1:18090'
$env:MEDIAD_DATA_DIR = Join-Path $previewDir 'service-state'
$env:MEDIAD_RESULT_DIR = Join-Path $previewDir 'staged-results'
$env:MEDIAD_CONFIG_PATH = Join-Path $previewDir 'service.json'
$env:MEDIAD_MANAGEMENT_TOKEN = $previewSettings.management_token
$env:MEDIAD_RELAY_SECRET = $previewSettings.relay_secret
$env:MEDIAD_DOWNLOAD_BASE_URL = 'http://127.0.0.1:18090'
$env:MEDIAD_PERSIST_CREDENTIALS = 'true'
New-Item -ItemType Directory -Force -Path $env:MEDIAD_DATA_DIR | Out-Null
$previewCallback = Join-Path $env:MEDIAD_DATA_DIR 'callback.json'
if (!(Test-Path -LiteralPath $previewCallback)) {
  @{ callback_url = 'http://127.0.0.1:18131/internal/media/result'; callback_secret = $previewSettings.callback_secret; enabled = $true; max_attempts = 10; backoff_base_seconds = 2 } | ConvertTo-Json | Set-Content -LiteralPath $previewCallback -Encoding utf8NoBOM
}
$started = @()
try {
  $started += Start-Process -WindowStyle Hidden -PassThru -FilePath (Join-Path $previewDir 'mediad.exe') -WorkingDirectory $previewRepo -RedirectStandardOutput (Join-Path $previewDir 'mediad.out.log') -RedirectStandardError (Join-Path $previewDir 'mediad.err.log')
  $started += Start-Process -WindowStyle Hidden -PassThru -FilePath (Join-Path $previewDir 'admin-preview.exe') -ArgumentList @('--settings', ('"' + $mainFile + '"')) -WorkingDirectory $previewRepo -RedirectStandardOutput (Join-Path $previewDir 'admin.out.log') -RedirectStandardError (Join-Path $previewDir 'admin.err.log')
  $previewNode = (Get-Command node).Source
  $previewVite = Join-Path $previewRepo 'frontend/node_modules/vite/bin/vite.js'
  $started += Start-Process -WindowStyle Hidden -PassThru -FilePath $previewNode -ArgumentList @(('"' + $previewVite + '"'), '--config', 'vite.config.ts', '--mode', 'media-preview', '--host', '127.0.0.1', '--port', '3000', '--strictPort') -WorkingDirectory (Join-Path $previewRepo 'frontend') -RedirectStandardOutput (Join-Path $previewDir 'frontend.out.log') -RedirectStandardError (Join-Path $previewDir 'frontend.err.log')
} finally {
  @($started | ForEach-Object { $proc = Get-CimInstance Win32_Process -Filter "ProcessId = $($_.Id)"; @{ id = $_.Id; path = $proc.ExecutablePath; created = $proc.CreationDate.ToUniversalTime().ToString('o') } }) | ConvertTo-Json | Set-Content -LiteralPath $pidFile -Encoding utf8NoBOM
}
Write-Output 'Local test page: http://127.0.0.1:3000/admin/media-service'
Write-Output 'Login with any local test email and password. Configuration and logs are in .dev/media-preview.'
