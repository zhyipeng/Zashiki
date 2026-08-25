# Adds or removes the current installer directory from the *user* PATH.
# Keeps all other entries untouched (including %VAR% style entries).
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File path_helper.ps1 -Operation add -PathDir "..."
#   powershell -ExecutionPolicy Bypass -File path_helper.ps1 -Operation remove -PathDir "..."
param(
    [ValidateSet('add', 'remove')]
    [Parameter(Mandatory = $true)]
    [string]$Operation,

    [Parameter(Mandatory = $true)]
    [string]$PathDir
)

$ErrorActionPreference = 'Stop'

if (-not [string]::IsNullOrWhiteSpace($PathDir)) {
    $PathDir = $PathDir.Trim().TrimEnd('\')
}
if ([string]::IsNullOrWhiteSpace($PathDir)) {
    throw 'PathDir is required.'
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($userPath -split ';' | Where-Object { $_ -ne '' })
$alreadyPresent = @($parts | Where-Object { $_.TrimEnd('\') -eq $PathDir }).Count -gt 0
$parts = @($parts | Where-Object { $_.TrimEnd('\') -ne $PathDir })

switch ($Operation) {
    'add' {
        if ($alreadyPresent) {
            Write-Host "$PathDir already on user PATH."
            return
        }
        [Environment]::SetEnvironmentVariable('Path', (($parts + $PathDir) -join ';'), 'User')
        Write-Host "Added $PathDir to user PATH."
    }
    'remove' {
        [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
        Write-Host "Removed $PathDir from user PATH."
    }
}
