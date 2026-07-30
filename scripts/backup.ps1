param(
    [string]$Destination = ""
)

$ErrorActionPreference = "Stop"
$rootDir = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($Destination)) {
    $Destination = Join-Path $rootDir "backups"
}

$resolvedDestination = [System.IO.Path]::GetFullPath($Destination)
[System.IO.Directory]::CreateDirectory($resolvedDestination) | Out-Null
$timestamp = [DateTime]::UtcNow.ToString("yyyyMMdd_HHmmss")
$outputPath = Join-Path $resolvedDestination "app_$timestamp.db"

Set-Location $rootDir
& docker compose cp "backend:/app/data/backups/app.db" $outputPath
if ($LASTEXITCODE -ne 0) {
    throw "Backup export failed. Ensure the stack is running and automatic backup is enabled."
}

Write-Host "Backup exported to: $outputPath"
