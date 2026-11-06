#Requires -RunAsAdministrator
[CmdletBinding()]
param([switch]$RemoveStatus)
$ErrorActionPreference = 'Stop'
foreach ($name in @('TimeSync-FiveMinutes', 'TimeSync-OnBoot')) {
    $task = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
    if ($task) {
        Unregister-ScheduledTask -TaskName $name -Confirm:$false -ErrorAction Stop
    }
}
$installDir = Join-Path $env:ProgramFiles 'TimeSync'
if (Test-Path -LiteralPath $installDir) {
    Remove-Item -LiteralPath $installDir -Recurse -Force
}
if ($RemoveStatus) {
    $statusDir = Join-Path $env:ProgramData 'TimeSync'
    if (Test-Path -LiteralPath $statusDir) {
        Remove-Item -LiteralPath $statusDir -Recurse -Force
    }
}
Write-Host 'TimeSync uninstalled. Windows Time service settings were not modified.'
