# build/windows/uninstall.ps1
# Removes the Zashiki app and detached PATH launcher from %LOCALAPPDATA%\zashiki
# and drops the managed directories from the *user* PATH. No admin rights needed.
#
# Usage: powershell -File uninstall.ps1 [-AppName <Zashiki.exe>]
param(
    [string]$AppName = "Zashiki.exe"
)

$ErrorActionPreference = 'Stop'

$appDir = Join-Path $env:LOCALAPPDATA 'zashiki\app'
$launcherDir = Join-Path $env:LOCALAPPDATA 'zashiki\launcher'
$legacyDir = Join-Path $env:LOCALAPPDATA 'zashiki\bin'
Remove-Item -Force -LiteralPath (Join-Path $appDir $AppName) -ErrorAction SilentlyContinue
Remove-Item -Force -LiteralPath (Join-Path $launcherDir 'zashiki.cmd') -ErrorAction SilentlyContinue
Remove-Item -Force -LiteralPath (Join-Path $legacyDir $AppName) -ErrorAction SilentlyContinue

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$managedDirs = @($launcherDir, $legacyDir) | ForEach-Object { $_.Trim().TrimEnd('\') }
$parts = @(
    $path -split ';' |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        ForEach-Object { $_.Trim() } |
        Where-Object { $managedDirs -notcontains $_.TrimEnd('\') }
)
[Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')

Write-Host "Removed zashiki from $appDir."
