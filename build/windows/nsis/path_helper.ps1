# Adds or removes the detached launcher directory from the *user* PATH.
# Keeps all other entries untouched (including %VAR% style entries).
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File path_helper.ps1 -Operation add -PathDir "..." [-LegacyPathDir "..."]
#   powershell -ExecutionPolicy Bypass -File path_helper.ps1 -Operation remove -PathDir "..." [-LegacyPathDir "..."]
param(
    [ValidateSet('add', 'remove')]
    [Parameter(Mandatory = $true)]
    [string]$Operation,

    [Parameter(Mandatory = $true)]
    [string]$PathDir,

    [string]$LegacyPathDir = ""
)

$ErrorActionPreference = 'Stop'

if (-not [string]::IsNullOrWhiteSpace($PathDir)) {
    $PathDir = $PathDir.Trim().TrimEnd('\')
}
if (-not [string]::IsNullOrWhiteSpace($LegacyPathDir)) {
    $LegacyPathDir = $LegacyPathDir.Trim().TrimEnd('\')
}
if ([string]::IsNullOrWhiteSpace($PathDir)) {
    throw 'PathDir is required.'
}

$managedDirs = @($PathDir, $LegacyPathDir) |
    Where-Object { -not [string]::IsNullOrWhiteSpace($_) }

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @(
    $userPath -split ';' |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        ForEach-Object { $_.Trim() } |
        Where-Object { $managedDirs -notcontains $_.TrimEnd('\') }
)

switch ($Operation) {
    'add' {
        [Environment]::SetEnvironmentVariable('Path', ((@($PathDir) + $parts) -join ';'), 'User')
        Write-Host "Added $PathDir to user PATH."
    }
    'remove' {
        [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
        Write-Host "Removed $PathDir from user PATH."
    }
}
