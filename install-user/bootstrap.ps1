
# bootstrap.ps1 — flujo zero-touch para Windows nativo (PowerShell).
#
# Lo que hace:
#   1. Baja el binario domain-install.exe ya publicado en la Release y lo verifica
#      contra SHA256SUMS.txt. Sin Go y sin compilar.
#   2. Si no hay asset para esta arquitectura, no hay red, o el checksum no cuadra,
#      cae al camino de siempre: detecta Go (>= 1.22), lo baja local si falta, compila.
#   3. Ejecuta el binario pasando los args que recibió este script.
#
# Cero dependencias además de PowerShell 5+ (viene con Windows 10+). Get-FileHash existe
# desde PowerShell 4.0, así que la verificación no necesita ningún binario externo.
#
# Uso:
#   .\bootstrap.ps1
#   .\bootstrap.ps1 -Url http://1.2.3.4 -Email u@x.cl -ApiKey domk_live_xxx
#   .\bootstrap.ps1 -Uninstall
#   .\bootstrap.ps1 -DryRun
#
# Para forzar la compilación desde el código: $env:DOMAIN_INSTALL_FROM_SOURCE = "1"
# Para apuntar a otro repo de releases:      $env:DOMAIN_REPO = "usuario/repo"
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
# LOCALAPPDATA y TEMP solo existen en Windows. Resolverlos con Join-Path directo aborta el
# script ENTERO en PowerShell Core sobre Linux ("Cannot bind argument to parameter 'Path'
# because it is null"), y eso es justamente lo que impediría testear este archivo sin una
# máquina Windows. GetTempPath() devuelve %TEMP% en Windows: el comportamiento real no cambia.
$GoInstallDir = if ($env:LOCALAPPDATA) { Join-Path $env:LOCALAPPDATA "go-domain" }
                else { Join-Path ([IO.Path]::GetTempPath()) "go-domain" }

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

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$ExePath   = Join-Path $ScriptDir "domain-install.exe"

# Los args se arman una sola vez porque hay DOS caminos que terminan ejecutando el binario:
# el descargado y el compilado. Duplicar el armado es como divergen.
$exeArgs = @()
if ($Url)       { $exeArgs += "--url";     $exeArgs += $Url }
if ($Email)     { $exeArgs += "--email";   $exeArgs += $Email }
if ($ApiKey)    { $exeArgs += "--api-key"; $exeArgs += $ApiKey }
if ($Uninstall) { $exeArgs += "--uninstall" }
if ($DryRun)    { $exeArgs += "--dry-run" }

function Invoke-DomainInstall {
  Write-Step "Ejecutando domain-install.exe $($exeArgs -join ' ')"
  & $ExePath @exeArgs
  exit $LASTEXITCODE
}

# ---------- binario publicado (DOMAINSERV-271; paridad con bootstrap.sh:106-152) ----------
# Bajarlo evita instalar Go y compilar, pero sobre todo evita que el cliente salga sin versión:
# un binario compilado acá se declara 'dev' y queda FUERA de la comparación por diseño, así que
# nunca recibiría el aviso de actualización. El de la release trae su -X puesto.
#
# La descarga es ANÓNIMA y esa es una restricción dura, no una preferencia: hay usuarios que
# solo clonan la solución y la instalan, sin cuenta de GitHub. Invoke-WebRequest sin headers
# de auth.
#
# /releases/latest/download resuelve al último tag publicado, así que el script no hardcodea
# ningún número de versión.
$DomainRepo = if ($env:DOMAIN_REPO) { $env:DOMAIN_REPO } else { "nunezlagos/domain" }
$Asset      = "domain-install-windows-$arch.exe"
$ReleaseUrl = "https://github.com/$DomainRepo/releases/latest/download"

if ($env:DOMAIN_INSTALL_FROM_SOURCE -ne "1") {
  Write-Step "Bajando el binario publicado ($Asset)"
  # Windows PowerShell 5.1 negocia TLS 1.0 por defecto en varias instalaciones y github.com
  # solo acepta 1.2+: sin esta linea la descarga falla siempre y el sintoma es "compila como antes".
  try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch {}
  $DL = New-Item -ItemType Directory -Force -Path (Join-Path ([IO.Path]::GetTempPath()) ("domain-dl-" + [Guid]::NewGuid()))
  try {
    $assetPath = Join-Path $DL $Asset
    $sumsPath  = Join-Path $DL "SHA256SUMS.txt"
    Invoke-WebRequest -Uri "$ReleaseUrl/$Asset" -OutFile $assetPath -UseBasicParsing -TimeoutSec 120
    Invoke-WebRequest -Uri "$ReleaseUrl/SHA256SUMS.txt" -OutFile $sumsPath -UseBasicParsing -TimeoutSec 60

    # SHA256SUMS.txt lo genera `sha256sum domain-install-*` en release-installer.yml, o sea
    # "<hash>  <archivo>". Se busca la línea del asset en vez de asumir su posición.
    $linea = Get-Content -LiteralPath $sumsPath |
             Where-Object { $_ -match "\s\*?$([regex]::Escape($Asset))\s*$" } |
             Select-Object -First 1
    if (-not $linea) { throw "SHA256SUMS.txt no lista $Asset" }
    $esperado = ($linea -split '\s+')[0]
    $real = (Get-FileHash -Algorithm SHA256 -LiteralPath $assetPath).Hash
    # Get-FileHash devuelve el hex en MAYÚSCULAS y sha256sum lo escribe en minúsculas: la
    # comparación TIENE que ser case-insensitive (-ine) o falla siempre y el script cae a
    # compilar sin que nadie entienda por qué.
    if ($real -ine $esperado) { throw "checksum distinto: esperado $esperado, obtenido $real" }

    Move-Item -Force -LiteralPath $assetPath -Destination $ExePath
    Write-OK "binario verificado contra SHA256SUMS.txt — sin Go, sin compilar"
    Remove-Item -Recurse -Force $DL -ErrorAction SilentlyContinue
    Invoke-DomainInstall
  } catch {
    # Que caiga acá es esperable: una arquitectura sin binario publicado, sin red, o un tag
    # cuyo workflow de release falló. Se informa y se sigue por el camino de siempre.
    Write-Warn- "no se pudo bajar o verificar el binario publicado — se compila desde el código"
    Write-Info $_.Exception.Message
    if ($arch -eq "arm64") {
      Write-Info "release-installer.yml todavía no publica windows/arm64: compilar es el camino esperado, no un fallo de red"
    }
  } finally {
    Remove-Item -Recurse -Force $DL -ErrorAction SilentlyContinue
  }
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
  $tmp = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ("go-bootstrap-" + [Guid]::NewGuid()))
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
Set-Location $ScriptDir

# Se compila SIN -X a propósito: un binario hecho desde el código fuente no es una release, y
# declararse 'dev' es lo que lo deja fuera de la comparación de versiones. Mismo criterio que
# bootstrap.sh y que install-user/Makefile.
Write-Step "Compilando domain-install.exe"
$env:CGO_ENABLED = "0"
& go build -ldflags "-s -w" -o domain-install.exe .
if ($LASTEXITCODE -ne 0) {
  Write-Fail "fallo de build"
  exit 1
}
$binSize = (Get-Item $ExePath).Length / 1MB
Write-OK ("binario listo: $ExePath ({0:N1} MB)" -f $binSize)

Invoke-DomainInstall


