# Register niagara-git in a station repository and merge the starter templates.
# Usage, from the station repo or with its path:
#   powershell -ExecutionPolicy Bypass -File .\install.ps1
#   powershell -ExecutionPolicy Bypass -File C:\tools\niagara-git\install.ps1 D:\stations\demoPlant
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Main = Join-Path $Root "niagara_git\__main__.py"

if ($env:PYTHON) {
    $Python = $env:PYTHON
} elseif (Get-Command python -ErrorAction SilentlyContinue) {
    $Python = "python"
} elseif (Get-Command python3 -ErrorAction SilentlyContinue) {
    $Python = "python3"
} else {
    Write-Error "niagara-git: Python 3.8+ is required on PATH (or set PYTHON)."
    exit 1
}

& $Python $Main install @args
exit $LASTEXITCODE
