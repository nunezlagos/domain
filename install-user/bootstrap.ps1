# bootstrap.ps1 — flujo zero-touch para Windows nativo (PowerShell).
#
# Lo que hace:
#   1. Detecta si Go (>= 1.22) está instalado.
#   2. Si NO está, baja Go oficial a %LOCALAPPDATA%\go-domain\ (sin admin).
#   3. Compila el binario domain-install.exe.
#   4. Lo ejecuta pasando los args que recibió este script.
#
# Cero dependencias además de PowerShell 5+ (viene con Windows 10+).
#
# Uso:
#   .\bootstrap.ps1
#   .\bootstrap.ps1 -Url http://1.2.3.4 -Email u@x.cl -ApiKey domk_live_xxx
#   .\bootstrap.ps1 -Uninstall
#   .\bootstrap.ps1 -DryRun
#
# Si Windows bloquea ejecución de scripts:
#   Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned

[CmdletBinding()]
param(
  [string]$Url,
  [string]$Email,
  [string]$ApiKey,
  [switch]$Uninstall,
  [switch]$DryRun
)

$ErrorActionPreference = "Stop"
$GoVersion = "1.22.6"
$GoInstallDir = Join-Path $env:LOCALAPPDATA "go-domain"

function Write-Step($msg)  { Write-Host ""; Write-Host "==> $msg" -ForegroundColor White }
function Write-OK($msg)    { Write-Host "    ✓ $msg" -ForegroundColor Green }
function Write-Warn-($msg) { Write-Host "    ! $msg" -ForegroundColor Yellow }
function Write-Fail($msg)  { Write-Host "    ✗ $msg" -ForegroundColor Red }
function Write-Info($msg)  { Write-Host "    · $msg" -ForegroundColor DarkGray }

# ---------- detectar arquitectura ----------
$arch = if ([Environment]::Is64BitOperatingSystem) {
  if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
  Write-Fail "Windows 32-bit no soportado"
  exit 1
}

# ---------- detectar Go ----------
$goOk = $false
$goCmd = Get-Command go -ErrorAction SilentlyContinue
if ($goCmd) {
  $verRaw = (go version) -split ' ' | Select-Object -Index 2
  $ver = $verRaw -replace '^go',''
  $parts = $ver -split '\.'
  if ([int]$parts[0] -ge 1 -and [int]$parts[1] -ge 22) {
    Write-Info "Go encontrado: $($goCmd.Source) (version $ver)"
    $goOk = $true
  } else {
    Write-Warn- "Go $ver detectado, pero necesitamos >= 1.22. Voy a bajar uno local."
  }
}

if (-not $goOk -and (Test-Path "$GoInstallDir\bin\go.exe")) {
  $env:PATH = "$GoInstallDir\bin;$env:PATH"
  $ver = (go version) -split ' ' | Select-Object -Index 2
  Write-OK "Reusando Go local previamente bajado: $ver"
  $goOk = $true
}

# ---------- instalar Go si falta ----------
if (-not $goOk) {
  Write-Step "Bajando Go $GoVersion a $GoInstallDir"
  $zipName = "go$GoVersion.windows-$arch.zip"
  $url = "https://go.dev/dl/$zipName"
  $tmp = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ("go-bootstrap-" + [Guid]::NewGuid()))
  try {
    Write-Info "URL: $url"
    $zipPath = Join-Path $tmp $zipName
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
    $size = (Get-Item $zipPath).Length / 1MB
    Write-OK ("zip bajado ({0:N1} MB)" -f $size)

    if (Test-Path $GoInstallDir) { Remove-Item -Recurse -Force $GoInstallDir }
    New-Item -ItemType Directory -Path $GoInstallDir | Out-Null
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force

    # El zip tiene un dir top-level "go/" — mover su contenido a $GoInstallDir
    $extracted = Join-Path $tmp "go"
    Get-ChildItem -Path $extracted -Force | Move-Item -Destination $GoInstallDir

    Write-OK "Go instalado en $GoInstallDir (sin admin, sin tocar PATH global)"
    $env:PATH = "$GoInstallDir\bin;$env:PATH"
    Write-Info "PATH actualizado para esta sesión: $GoInstallDir\bin"
    Write-Info "Para usarlo en otras sesiones: setx PATH `"%LOCALAPPDATA%\go-domain\bin;%PATH%`""
  } finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
  }
}

# ---------- build ----------
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
Set-Location $ScriptDir

Write-Step "Compilando domain-install.exe"
$env:CGO_ENABLED = "0"
& go build -ldflags "-s -w" -o domain-install.exe .
if ($LASTEXITCODE -ne 0) {
  Write-Fail "fallo de build"
  exit 1
}
$binSize = (Get-Item .\domain-install.exe).Length / 1MB
Write-OK ("binario listo: $ScriptDir\domain-install.exe ({0:N1} MB)" -f $binSize)

# ---------- run ----------
$exeArgs = @()
if ($Url)       { $exeArgs += "--url";     $exeArgs += $Url }
if ($Email)     { $exeArgs += "--email";   $exeArgs += $Email }
if ($ApiKey)    { $exeArgs += "--api-key"; $exeArgs += $ApiKey }
if ($Uninstall) { $exeArgs += "--uninstall" }
if ($DryRun)    { $exeArgs += "--dry-run" }

Write-Step "Ejecutando domain-install.exe $($exeArgs -join ' ')"
& .\domain-install.exe @exeArgs
exit $LASTEXITCODE
