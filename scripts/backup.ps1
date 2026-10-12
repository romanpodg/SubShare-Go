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

function Resolve-BackendExportPath([string]$path) {
    if ($path.StartsWith('/')) { return $path }
    $backendPath = & docker compose exec -T backend sh -ec 'printf "%s/%s" "$PWD" "$1"' sh $path
    if ($LASTEXITCODE -ne 0) { throw "Could not resolve the configured path in the running backend." }
    return ($backendPath -join "`n")
}

Set-Location $rootDir
$backendKeyringPath = & docker compose exec -T backend sh -ec 'test -z "${PROFILE_ENCRYPTION_KEYRING_JSON:-}" && printf "%s" "${PROFILE_ENCRYPTION_KEYRING_FILE:-}"'
if ($LASTEXITCODE -ne 0) {
    throw "An active file-backed keyring is required. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting an inline-keyring database."
}
$configuredKeyringPath = ($backendKeyringPath -join "`n").Trim()
if ([string]::IsNullOrWhiteSpace($configuredKeyringPath)) {
    throw "No active file-backed keyring is configured for export."
}
& docker compose exec -T backend sh -ec 'test -f "$1"' sh $configuredKeyringPath
if ($LASTEXITCODE -ne 0) {
    throw "No file-backed keyring exists in the data volume. Preserve PROFILE_ENCRYPTION_KEYRING_JSON securely before exporting this database."
}
$configuredKeyringPath = Resolve-BackendExportPath $configuredKeyringPath
$backendBackupPath = & docker compose exec -T backend sh -ec 'printf "%s" "${BACKUP_PATH:-}"'
if ($LASTEXITCODE -ne 0) {
    throw "Could not read BACKUP_PATH from the running backend."
}
$configuredBackupPath = ($backendBackupPath -join "`n").Trim()
if ([string]::IsNullOrWhiteSpace($configuredBackupPath)) {
    throw "Automatic backup is disabled (BACKUP_PATH is empty)."
}
$configuredBackupPath = Resolve-BackendExportPath $configuredBackupPath
& docker compose cp "backend:$configuredBackupPath" $outputPath
if ($LASTEXITCODE -ne 0) {
    throw "Backup export failed. Ensure the stack is running and automatic backup is enabled."
}
& docker compose cp "backend:$configuredKeyringPath" $keyringOutputPath
if ($LASTEXITCODE -ne 0) {
    throw "Keyring export failed. The database backup is not independently restorable without its matching keyring."
}

Write-Host "Backup exported to: $outputPath"
Write-Host "Matching encryption keyring exported to: $keyringOutputPath"
