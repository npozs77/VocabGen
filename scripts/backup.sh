#!/usr/bin/env bash
# Backup the vocabgen SQLite database.
# Usage:
#   ./scripts/backup.sh                    # backup to default location
#   ./scripts/backup.sh /path/to/backup    # backup to custom directory
#
# Cron example (daily at 2am):
#   0 2 * * * /path/to/vocabgen/scripts/backup.sh >> /tmp/vocabgen-backup.log 2>&1

set -euo pipefail

DB_PATH="${VOCABGEN_DB:-$HOME/.vocabgen/vocabgen.db}"
BACKUP_DIR="${1:-$HOME/.vocabgen/backups}"
TIMESTAMP=$(date -u +%Y-%m-%dT%H-%M-%S)
BACKUP_FILE="$BACKUP_DIR/vocabgen.db.$TIMESTAMP.bak"
MAX_BACKUPS="${VOCABGEN_MAX_BACKUPS:-30}"

if [ ! -f "$DB_PATH" ]; then
    echo "ERROR: Database not found at $DB_PATH"
    exit 1
fi

mkdir -p "$BACKUP_DIR"

# Use sqlite3 .backup for a consistent copy (safe even if DB is in use)
if command -v sqlite3 &>/dev/null; then
    sqlite3 "$DB_PATH" ".backup '$BACKUP_FILE'"
else
    cp "$DB_PATH" "$BACKUP_FILE"
fi

echo "Backup created: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"

# Prune old backups, keep the most recent $MAX_BACKUPS
BACKUP_COUNT=$(ls -1 "$BACKUP_DIR"/vocabgen.db.*.bak 2>/dev/null | wc -l | tr -d ' ')
if [ "$BACKUP_COUNT" -gt "$MAX_BACKUPS" ]; then
    REMOVE_COUNT=$((BACKUP_COUNT - MAX_BACKUPS))
    ls -1t "$BACKUP_DIR"/vocabgen.db.*.bak | tail -n "$REMOVE_COUNT" | xargs rm -f
    echo "Pruned $REMOVE_COUNT old backup(s), keeping $MAX_BACKUPS"
fi
