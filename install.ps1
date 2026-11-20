# Locate or download the niagara-git binary and register it in a station repo.
# Usage, from the station repo or with its path:
#   powershell -ExecutionPolicy Bypass -File .\install.ps1
#   powershell -ExecutionPolicy Bypass -File C:\tools\niagara-git\install.ps1 D:\stations\demoPlant
$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path

function Get-NiagaraGitRepo {
    if ($env:NIAGARA_GIT_REPO) { return $env:NIAGARA_GIT_REPO }
    try {
        $origin = git -C $Root remote get-url origin 2>$null
        if ($origin -match "github\.com[:/]([^/]+/[^/]+?)(?:\.git)?$") {
            return $Matches[1]
        }
    } catch {
    }
    return "bew1769/niagara-git"
}

function Download-NiagaraGit {
    $arch = switch -Regex ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64|X64" { "amd64" }
        "ARM64" { "arm64" }
        default { "" }
    }
    if ($arch -ne "amd64") {
        Write-Error "niagara-git publishes a Windows amd64 binary. This machine is $($env:PROCESSOR_ARCHITECTURE)."
        exit 1
    }
    $repo = Get-NiagaraGitRepo
    $version = if ($env:NIAGARA_GIT_VERSION) { $env:NIAGARA_GIT_VERSION } else { "latest" }
    $asset = "niagara-git-windows-amd64.exe"
    if ($version -eq "latest") {
        $url = "https://github.com/$repo/releases/latest/download/$asset"
    } else {
        $url = "https://github.com/$repo/releases/download/$version/$asset"
    }
    $cache = Join-Path $env:LOCALAPPDATA "niagara-git"
    New-Item -ItemType Directory -Force -Path $cache | Out-Null
    $dest = Join-Path $cache "niagara-git.exe"
    Invoke-WebRequest -Uri $url -OutFile $dest
    return $dest
}

function Resolve-NiagaraGit {
    if ($env:NIAGARA_GIT -and (Test-Path -LiteralPath $env:NIAGARA_GIT)) {
        return $env:NIAGARA_GIT
    }
    foreach ($name in @("niagara-git.exe", "niagara-git")) {
        $candidate = Join-Path $Root $name
        if (Test-Path -LiteralPath $candidate) { return $candidate }
    }
    $cmd = Get-Command niagara-git -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    return Download-NiagaraGit
}

$Bin = Resolve-NiagaraGit
& $Bin install @args
exit $LASTEXITCODE
