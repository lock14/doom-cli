<#
.SYNOPSIS
    Installs DoomRunner configurations and presets for Windows.
.DESCRIPTION
    Deploys DoomRunner options.json and presets.json into %LOCALAPPDATA% and %APPDATA%,
    creating timestamped backups of existing files. Automatically defaults to the drive letter
    the setup process is being executed from, or supports custom drive letters and WAD directories.
.PARAMETER BaseDrive
    The base drive letter (e.g. 'C:', 'D:', or 'E:'). Defaults to the drive the setup process
    is being executed from.
.PARAMETER WadsDir
    Custom path to the WADs directory (e.g. 'D:\Games\Doom WADS').
.EXAMPLE
    .\setup.ps1
.EXAMPLE
    .\setup.ps1 -BaseDrive "D:"
.EXAMPLE
    .\setup.ps1 -WadsDir "D:\Games\Doom WADS"
#>
param (
    [string]$BaseDrive = "",
    [string]$WadsDir = ""
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

$TargetDirs = @(
    "$env:LOCALAPPDATA\DoomRunner",
    "$env:APPDATA\DoomRunner"
)

$DataDirs = @(
    "$env:LOCALAPPDATA\doom-configs",
    "$env:APPDATA\doom-configs"
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

# If BaseDrive was not specified, default to the drive the setup process is being executed from
if ([string]::IsNullOrWhiteSpace($BaseDrive)) {
    $DetectedDrive = ""
    if (-not [string]::IsNullOrWhiteSpace($ScriptDir)) {
        $DetectedDrive = Split-Path -Qualifier $ScriptDir
    }
    if ([string]::IsNullOrWhiteSpace($DetectedDrive)) {
        $DetectedDrive = Split-Path -Qualifier (Get-Location).Path
    }
    if ([string]::IsNullOrWhiteSpace($DetectedDrive)) {
        $DetectedDrive = "C:"
    }
    $BaseDrive = $DetectedDrive
}

$NormalizedDrive = $BaseDrive.Replace('\', '/').TrimEnd('/')
if ($NormalizedDrive -match '^[a-zA-Z]$') {
    $NormalizedDrive = "$NormalizedDrive`:"
}
$NormalizedDrive = $NormalizedDrive.ToUpper()

Write-Host "Setting up Doom configurations for Windows (Base Drive: $NormalizedDrive)..."

# Read and optionally remap base drive for DoomRunner options
$SourceOptions = Join-Path $ScriptDir "DoomRunner\windows\options.json"
$OptionsContent = Get-Content -Path $SourceOptions -Raw

if ($NormalizedDrive -ne "E:") {
    $OptionsContent = $OptionsContent.Replace("E:/", "$NormalizedDrive/")
}

if ($WadsDir -ne "") {
    $NormalizedWadsDir = $WadsDir.Replace('\', '/').TrimEnd('/')
    $TargetWadsPath = "$NormalizedDrive/Doom WADS"
    $OptionsContent = $OptionsContent.Replace($TargetWadsPath, $NormalizedWadsDir)
    $OptionsContent = $OptionsContent.Replace("E:/Doom WADS", $NormalizedWadsDir)
}

foreach ($Dir in $TargetDirs) {
    $TargetFile = Join-Path $Dir "options.json"
    Install-ConfigWithBackup -Content $OptionsContent -Destination $TargetFile
}

# Deploy declarative presets.json data
$SourcePresets = Join-Path $ScriptDir "data\presets.json"
$PresetsContent = Get-Content -Path $SourcePresets -Raw
foreach ($Dir in $DataDirs) {
    $TargetFile = Join-Path $Dir "presets.json"
    Install-ConfigWithBackup -Content $PresetsContent -Destination $TargetFile
}

if (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host "Compiling native doom CLI (doom.exe)..."
    $DoomBinDir = "$env:LOCALAPPDATA\Programs\Doom\bin"
    if (-not (Test-Path $DoomBinDir)) { New-Item -ItemType Directory -Path $DoomBinDir -Force | Out-Null }
    $OutExe = Join-Path $DoomBinDir "doom.exe"
    try {
        & go build -o $OutExe (Join-Path $ScriptDir "cmd\doom")
        Write-Host "✓ Installed doom.exe to $OutExe"
    } catch {
        Write-Host "Warning: could not compile doom.exe: $_"
    }
}

Write-Host "Setup complete!"

