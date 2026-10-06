# SQLite startup failures and recovery

SubShare initializes SQLite once per startup. If initialization fails, startup stops and returns the database error. It never deletes `DB_PATH-wal` or `DB_PATH-shm` to force a retry. A later startup lets SQLite perform its own recovery. Normal migrations, WAL/DELETE configuration and SQLite's existing WAL-to-DELETE fallback still apply.

The WAL can contain committed accounts and encrypted profiles that have not reached the main database file. SQLite treats it as part of the database; separating or deleting it can lose committed transactions. A successful database close may checkpoint the WAL, but a crash or failed close does not establish that it is disposable. See [SQLite's WAL file documentation](https://sqlite.org/wal.html#the_wal_file).

## Operator recovery

1. Read the backend startup error. Check free disk space, filesystem errors, directory permissions and other processes using the same database. For a lock error, identify the lock holder before restarting.
2. Stop all processes using that database before making a filesystem copy. Retain the main database, every existing `-wal`, `-shm` and rollback journal file, the matching profile encryption keyring, and the fingerprint keys. Protect the copy as credential-bearing state. Do not copy only the main file or replace the keyring. For a running database, use a SQLite-supported backup mechanism rather than copying files independently; SQLite documents the risks of [inconsistent live backups](https://sqlite.org/howtocorrupt.html#_backup_or_restore_while_a_transaction_is_active).
3. Resolve the underlying filesystem or locking problem, then retry startup with the original database and keys. SQLite decides whether to checkpoint or remove sidecars. Changing `DB_JOURNAL_MODE` does not authorize manual WAL deletion.
4. If startup still fails, investigate an isolated copy or restore a verified backup with its matching keyring. Keep the original files available for recovery. Initialization can apply migrations before a later error, so a failed startup is not a guarantee that every database byte is unchanged.

There is no automatic destructive repair or sidecar-cleanup command. The regression tests use disposable databases and injected errors; they do not reproduce every operating-system I/O or permission failure.
