#Requires -RunAsAdministrator
[CmdletBinding()]
param([string]$BinaryPath = (Join-Path $PSScriptRoot 'time-sync.exe'))
$ErrorActionPreference = 'Stop'
$taskName = 'TimeSync-FiveMinutes'
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
$escaped = [System.Security.SecurityElement]::Escape($target)
$start = (Get-Date).AddMinutes(1).ToString('yyyy-MM-ddTHH:mm:ss')
$xml = @"
<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Synchronize the Windows UTC clock with multiple NTP servers every five minutes.</Description></RegistrationInfo>
  <Triggers>
    <BootTrigger><Enabled>true</Enabled><Delay>PT30S</Delay></BootTrigger>
    <CalendarTrigger>
      <Enabled>true</Enabled><StartBoundary>$start</StartBoundary>
      <Repetition><Interval>PT5M</Interval><StopAtDurationEnd>false</StopAtDurationEnd></Repetition>
      <ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>
    </CalendarTrigger>
  </Triggers>
  <Principals><Principal id="System"><UserId>S-1-5-18</UserId><LogonType>ServiceAccount</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <Enabled>true</Enabled>
    <ExecutionTimeLimit>PT1M</ExecutionTimeLimit>
  </Settings>
  <Actions Context="System"><Exec><Command>$escaped</Command><Arguments>-mode sync</Arguments><WorkingDirectory>$([System.Security.SecurityElement]::Escape($installDir))</WorkingDirectory></Exec></Actions>
</Task>
"@
$temp = Join-Path $env:TEMP ('time-sync-task-' + [guid]::NewGuid().ToString('N') + '.xml')
try {
    [System.IO.File]::WriteAllText($temp, $xml, (New-Object System.Text.UTF8Encoding($false)))
    & schtasks.exe /Create /TN $taskName /XML $temp /F
    if ($LASTEXITCODE -ne 0) { throw "Failed to create scheduled task (code $LASTEXITCODE)" }
} finally {
    Remove-Item -LiteralPath $temp -Force -ErrorAction SilentlyContinue
}
Start-ScheduledTask -TaskName $taskName
Write-Host "Installed: $target"
Write-Host "Task: $taskName (boot + every 5 minutes; runs as SYSTEM)"
Write-Host 'To inspect synchronization, read C:\ProgramData\TimeSync\status.json or run time-sync.exe -mode check.'
