#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [string]$BinaryPath,
    [string]$Interval = '1m',
    [switch]$SkipInitialRun
)
$ErrorActionPreference = 'Stop'

function ConvertTo-TaskInterval {
    param([Parameter(Mandatory)][string]$Value)

    $text = $Value.Trim()
    $duration = [TimeSpan]::Zero
    $match = [regex]::Match($text, '^([0-9]+)\s*([smhd])$', [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
    if ($match.Success) {
        $amount = [int64]$match.Groups[1].Value
        switch ($match.Groups[2].Value.ToLowerInvariant()) {
            's' { $duration = [TimeSpan]::FromSeconds($amount) }
            'm' { $duration = [TimeSpan]::FromMinutes($amount) }
            'h' { $duration = [TimeSpan]::FromHours($amount) }
            'd' { $duration = [TimeSpan]::FromDays($amount) }
        }
    } else {
        try {
            $duration = [TimeSpan]::Parse($text, [Globalization.CultureInfo]::InvariantCulture)
        } catch {
            throw "Invalid interval '$Value'. Use a number followed by m, h or d (for example 1m or 2h), or HH:MM:SS."
        }
    }

    if ($duration -lt [TimeSpan]::FromMinutes(1) -or $duration -gt [TimeSpan]::FromMinutes(1439) -or $duration.TotalMinutes -ne [math]::Floor($duration.TotalMinutes)) {
        throw 'Interval must be a whole number of minutes between 1 minute and 23 hours 59 minutes.'
    }

    [pscustomobject]@{ TimeSpan = $duration; Minutes = [int]$duration.TotalMinutes }
}

$intervalInfo = ConvertTo-TaskInterval $Interval
# Resolve the script directory after PowerShell binds parameters.
if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    $BinaryPath = Join-Path -Path $PSScriptRoot -ChildPath 'time-sync.exe'
}
$BinaryPath = [System.IO.Path]::GetFullPath($BinaryPath)
$intervalTaskName = 'TimeSync-Periodic'
$legacyIntervalTaskName = 'TimeSync-FiveMinutes'
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

# Remove the old task name when upgrading from releases that used it.
$legacyTask = Get-ScheduledTask -TaskName $legacyIntervalTaskName -ErrorAction SilentlyContinue
if ($legacyTask) {
    Unregister-ScheduledTask -TaskName $legacyIntervalTaskName -Confirm:$false -ErrorAction Stop
}

# Use native Windows Task Scheduler triggers. Launch schtasks.exe with one
# explicitly quoted command-line string so paths containing spaces remain intact
# in Windows PowerShell 5.1 and PowerShell 7.
$taskCommand = '\"' + $target + '\" -mode sync'
$scheduledTasksExe = Join-Path $env:SystemRoot 'System32\schtasks.exe'
$periodicTaskArgs = '/Create /TN "{0}" /TR "{1}" /SC MINUTE /MO {2} /RU SYSTEM /RL HIGHEST /F' -f $intervalTaskName, $taskCommand, $intervalInfo.Minutes
$periodicResult = Start-Process -FilePath $scheduledTasksExe -ArgumentList $periodicTaskArgs -Wait -PassThru -NoNewWindow
if ($periodicResult.ExitCode -ne 0) {
    throw "Failed to register synchronization task (exit code $($periodicResult.ExitCode))."
}
$startupArgs = '/Create /TN "{0}" /TR "{1}" /SC ONSTART /DELAY 0000:30 /RU SYSTEM /RL HIGHEST /F' -f $bootTaskName, $taskCommand
$startupResult = Start-Process -FilePath $scheduledTasksExe -ArgumentList $startupArgs -Wait -PassThru -NoNewWindow
if ($startupResult.ExitCode -ne 0) {
    throw "Failed to register startup task (exit code $($startupResult.ExitCode))."
}
if (-not $SkipInitialRun) {
    & schtasks.exe /Run /TN $intervalTaskName
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to start synchronization task (exit code $LASTEXITCODE)."
    }
}
Write-Host "Installed: $target"
Write-Host "Tasks: $intervalTaskName (every $($intervalInfo.TimeSpan)), $bootTaskName (at boot, delayed 30 seconds)."
Write-Host 'Both run as SYSTEM without a user sign-in.'
Write-Host 'Status: C:\ProgramData\TimeSync\status.json'
