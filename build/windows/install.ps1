# build/windows/install.ps1
# Copies the built Zashiki.exe into %LOCALAPPDATA%\zashiki\bin and adds that
# directory to the *user* PATH if it is not already present. No admin rights needed.
#
# Usage: powershell -File install.ps1 -Source <path\to\Zashiki.exe>
param(
    [Parameter(Mandatory = $true)]
    [string]$Source,

    [string]$AppName = "Zashiki.exe"
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $Source)) {
    throw "Source binary not found: $Source"
}

$dstDir = Join-Path $env:LOCALAPPDATA 'zashiki\bin'
New-Item -ItemType Directory -Force -Path $dstDir | Out-Null
Copy-Item -Force -LiteralPath $Source (Join-Path $dstDir $AppName)

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($path -split ';' | Where-Object { $_ })
if ($parts -notcontains $dstDir) {
    $newPath = if ($parts.Count -gt 0) { ($parts + $dstDir) -join ';' } else { $dstDir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "Added $dstDir to user PATH."
} else {
    Write-Host "$dstDir already on user PATH."
}

Write-Host "Installed zashiki to $dstDir."
Write-Host "Open a new terminal for PATH changes to take effect."
