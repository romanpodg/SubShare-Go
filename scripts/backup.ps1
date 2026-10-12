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
$keyringOutputPath = Join-Path $resolvedDestination "keyring_$timestamp.json"

Set-Location $rootDir
& docker compose exec -T backend sh -ec "test -f /app/data/keyring.json"
if ($LASTEXITCODE -ne 0) {
    throw "No file-backed keyring exists in the data volume. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting this database."
}
$backendBackupPath = & docker compose exec -T backend sh -ec 'printf "%s" "${BACKUP_PATH:-}"'
if ($LASTEXITCODE -ne 0) {
    throw "Could not read BACKUP_PATH from the running backend."
}
$configuredBackupPath = ($backendBackupPath -join "`n").Trim()
if ([string]::IsNullOrWhiteSpace($configuredBackupPath)) {
    throw "Automatic backup is disabled (BACKUP_PATH is empty)."
}
& docker compose cp "backend:$configuredBackupPath" $outputPath
if ($LASTEXITCODE -ne 0) {
    throw "Backup export failed. Ensure the stack is running and automatic backup is enabled."
}
& docker compose cp "backend:/app/data/keyring.json" $keyringOutputPath
if ($LASTEXITCODE -ne 0) {
    throw "Keyring export failed. The database backup is not independently restorable without its matching keyring."
}

Write-Host "Backup exported to: $outputPath"
Write-Host "Matching encryption keyring exported to: $keyringOutputPath"
