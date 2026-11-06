#Requires -RunAsAdministrator
[CmdletBinding()]
param([switch]$RemoveStatus)
$ErrorActionPreference = 'Stop'
$task = Get-ScheduledTask -TaskName 'TimeSync-FiveMinutes' -ErrorAction SilentlyContinue
if ($task) { Unregister-ScheduledTask -TaskName 'TimeSync-FiveMinutes' -Confirm:$false }
$installDir = Join-Path $env:ProgramFiles 'TimeSync'
if (Test-Path -LiteralPath $installDir) { Remove-Item -LiteralPath $installDir -Recurse -Force }
if ($RemoveStatus) {
    $statusDir = Join-Path $env:ProgramData 'TimeSync'
    if (Test-Path -LiteralPath $statusDir) { Remove-Item -LiteralPath $statusDir -Recurse -Force }
}
Write-Host 'TimeSync uninstalled. Windows Time service settings were not modified.'
