# Run the actual export script with a fake Docker command; no live stack access.
$ErrorActionPreference = 'Stop'
function docker {
    $taskArguments = @($args)
    $global:LASTEXITCODE = 0
    if ($taskArguments[1] -eq 'exec') {
        Invoke-BackupFixtureExec $taskArguments
        return
    }
    if ($taskArguments[1] -eq 'cp') {
        Invoke-BackupFixtureCopy $taskArguments
        return
    }
    throw 'Unexpected fake Docker command'
}

function Invoke-BackupFixtureExec([object[]]$taskArguments) {
    if (($taskArguments -join ' ') -match 'PROFILE_ENCRYPTION_KEYRING') {
        Invoke-BackupFixtureKeyring
        return
    }
    if (($taskArguments -join ' ') -notmatch 'BACKUP_PATH') {
        if ($env:SUBSHARE_BACKUP_TEST_FAILURE -eq 'keyring') { $global:LASTEXITCODE = 1 }
        return
    }
    if ($env:SUBSHARE_BACKUP_TEST_FAILURE -eq 'lookup') { $global:LASTEXITCODE = 1; return }
    Write-Output $env:SUBSHARE_BACKUP_TEST_PATH
}

function Invoke-BackupFixtureKeyring {
    if ($env:SUBSHARE_BACKUP_TEST_INLINE) { $global:LASTEXITCODE = 1; return }
    $path = $env:SUBSHARE_BACKUP_TEST_KEYRING_PATH
    if ($null -eq $path) { $path = '/app/data/keyring.json' }
    Write-Output $path
}

function Invoke-BackupFixtureCopy([object[]]$taskArguments) {
    $source = $taskArguments[2]
    [System.IO.File]::AppendAllText($env:SUBSHARE_BACKUP_TEST_CAPTURE, $source + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
    if ($env:SUBSHARE_BACKUP_TEST_FAILURE -eq 'database-copy' -and $source -ne 'backend:/app/data/keyring.json') { $global:LASTEXITCODE = 1; return }
    if ($env:SUBSHARE_BACKUP_TEST_FAILURE -eq 'keyring-copy' -and $source -eq 'backend:/app/data/keyring.json') { $global:LASTEXITCODE = 1; return }
    Set-Content -LiteralPath $taskArguments[3] -Value 'disposable export fixture'
}
try {
    & (Join-Path $env:SUBSHARE_BACKUP_TEST_ROOT 'scripts/backup.ps1') -Destination $env:SUBSHARE_BACKUP_TEST_DESTINATION
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}
