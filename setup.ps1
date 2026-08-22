param (
    [string]$BaseDrive = "E:"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

$TargetDirs = @(
    "$env:LOCALAPPDATA\DoomRunner",
    "$env:APPDATA\DoomRunner"
)

function Install-ConfigWithBackup {
    param (
        [string]$Content,
        [string]$Destination
    )

    $DestDir = Split-Path -Parent $Destination
    if (-not (Test-Path -Path $DestDir)) {
        New-Item -ItemType Directory -Path $DestDir -Force | Out-Null
    }

    if (Test-Path -Path $Destination) {
        $Timestamp = Get-Date -Format "yyyyMMddHHmmss"
        $BackupPath = "$Destination.bak.$Timestamp"
        Write-Host "Backing up existing $(Split-Path -Leaf $Destination) to $BackupPath"
        Copy-Item -Path $Destination -Destination $BackupPath -Force
    }

    Write-Host "Installing $(Split-Path -Leaf $Destination)..."
    Set-Content -Path $Destination -Value $Content -Encoding UTF8
}

Write-Host "Setting up Doom configurations for Windows..."

# Read and optionally remap base drive for DoomRunner options
$SourceOptions = Join-Path $ScriptDir "DoomRunner\windows\options.json"
$OptionsContent = Get-Content -Path $SourceOptions -Raw
if ($BaseDrive -ne "E:") {
    $NormalizedDrive = $BaseDrive.TrimEnd('\').TrimEnd('/')
    if (-not $NormalizedDrive.EndsWith(':')) {
        $NormalizedDrive = "$NormalizedDrive`:"
    }
    $OptionsContent = $OptionsContent.Replace("E:", $NormalizedDrive)
}

foreach ($Dir in $TargetDirs) {
    $TargetFile = Join-Path $Dir "options.json"
    Install-ConfigWithBackup -Content $OptionsContent -Destination $TargetFile
}

Write-Host "Setup complete!"
