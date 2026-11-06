#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [string]$BinaryPath,
    [switch]$SkipInitialRun
)
$ErrorActionPreference = 'Stop'

# Resolve the script directory after PowerShell binds parameters.
if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    $BinaryPath = Join-Path -Path $PSScriptRoot -ChildPath 'time-sync.exe'
}
$BinaryPath = [System.IO.Path]::GetFullPath($BinaryPath)
$intervalTaskName = 'TimeSync-FiveMinutes'
$bootTaskName = 'TimeSync-OnBoot'
$installDir = Join-Path $env:ProgramFiles 'TimeSync'
$target = Join-Path $installDir 'time-sync.exe'

if (-not (Test-Path -LiteralPath $BinaryPath -PathType Leaf)) {
    throw "Missing executable: $BinaryPath. Extract the release ZIP before installing."
}
# Domain-joined machines normally must obey their domain hierarchy for time.
if ((Get-CimInstance Win32_ComputerSystem).PartOfDomain) {
    throw 'Domain-joined PC detected. Do not replace domain time policy with this tool.'
}
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
# Program Files has inherited ACLs that prohibit modifications by ordinary users.
Copy-Item -LiteralPath $BinaryPath -Destination $target -Force

# Use documented schtasks.exe native triggers, not hand-authored Task Scheduler
# XML: various Windows builds reject XML principal and encoding combinations.
# MINUTE /MO 5 repeats without an end date; ONSTART also works without login.
$taskCommand = '"' + $target + '" -mode sync'
& schtasks.exe /Create /TN $intervalTaskName /TR $taskCommand /SC MINUTE /MO 5 /RU SYSTEM /RL HIGHEST /F
if ($LASTEXITCODE -ne 0) {
    throw "Failed to register five-minute task (exit code $LASTEXITCODE)."
}
& schtasks.exe /Create /TN $bootTaskName /TR $taskCommand /SC ONSTART /DELAY 0000:30 /RU SYSTEM /RL HIGHEST /F
if ($LASTEXITCODE -ne 0) {
    throw "Failed to register startup task (exit code $LASTEXITCODE)."
}
if (-not $SkipInitialRun) {
    & schtasks.exe /Run /TN $intervalTaskName
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to start synchronization task (exit code $LASTEXITCODE)."
    }
}
Write-Host "Installed: $target"
Write-Host "Tasks: $intervalTaskName (every five minutes), $bootTaskName (at boot, delayed 30 seconds)."
Write-Host 'Both run as SYSTEM without a user sign-in.'
Write-Host 'Status: C:\ProgramData\TimeSync\status.json'
