# build/windows/uninstall.ps1
# Removes Zashiki.exe from %LOCALAPPDATA%\zashiki\bin and drops that directory
# from the *user* PATH if present. No admin rights needed.
#
# Usage: powershell -File uninstall.ps1 [-AppName <Zashiki.exe>]
param(
    [string]$AppName = "Zashiki.exe"
)

$ErrorActionPreference = 'Stop'

$dstDir = Join-Path $env:LOCALAPPDATA 'zashiki\bin'
Remove-Item -Force -LiteralPath (Join-Path $dstDir $AppName) -ErrorAction SilentlyContinue

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($path -split ';' | Where-Object { $_ -ne $dstDir })
[Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')

Write-Host "Removed zashiki from $dstDir."
