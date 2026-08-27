# build/windows/install.ps1
# Copies the built Zashiki.exe into %LOCALAPPDATA%\zashiki\app and puts a
# detached `zashiki.cmd` launcher in %LOCALAPPDATA%\zashiki\launcher on the
# *user* PATH. No admin rights needed.
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

$appDir = Join-Path $env:LOCALAPPDATA 'zashiki\app'
$launcherDir = Join-Path $env:LOCALAPPDATA 'zashiki\launcher'
$legacyDir = Join-Path $env:LOCALAPPDATA 'zashiki\bin'
New-Item -ItemType Directory -Force -Path $appDir, $launcherDir | Out-Null
Copy-Item -Force -LiteralPath $Source (Join-Path $appDir $AppName)

$launcherPath = Join-Path $launcherDir 'zashiki.cmd'
$launcherContent = "@echo off`r`nstart `"`" /B `"%~dp0..\app\$AppName`" %*`r`n"
[System.IO.File]::WriteAllText($launcherPath, $launcherContent, [System.Text.Encoding]::ASCII)

# Remove the old direct-executable installation so it cannot win command lookup.
Remove-Item -Force -LiteralPath (Join-Path $legacyDir $AppName) -ErrorAction SilentlyContinue

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$managedDirs = @($launcherDir, $legacyDir) | ForEach-Object { $_.Trim().TrimEnd('\') }
$parts = @(
    $path -split ';' |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        ForEach-Object { $_.Trim() } |
        Where-Object { $managedDirs -notcontains $_.TrimEnd('\') }
)
$newPath = (@($launcherDir) + $parts) -join ';'
[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
Write-Host "Added $launcherDir to user PATH."

Write-Host "Installed zashiki to $appDir."
Write-Host "Open a new terminal for PATH changes to take effect."
