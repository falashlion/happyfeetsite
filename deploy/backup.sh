#!/bin/sh
# ══════════════════════════════════════════════════════════════════════════
# Nightly Postgres backup with rotation, and optional off-host upload.
#
# A backup that lives only on the machine it is backing up is not a backup —
# it survives a bad migration but not a lost VM. Set the S3_BACKUP_* variables
# to push a copy to Cloudflare R2 (10 GB free) or any S3-compatible store.
# ══════════════════════════════════════════════════════════════════════════
set -eu

BACKUP_DIR=/backups
KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
INTERVAL="${BACKUP_INTERVAL_SECONDS:-86400}"

log() { echo "[backup] $(date -u '+%Y-%m-%dT%H:%M:%SZ') $*"; }

run_backup() {
	stamp=$(date -u '+%Y%m%d-%H%M%S')
	file="$BACKUP_DIR/happyfeet-$stamp.sql.gz"

	log "dumping to $file"
	# --clean --if-exists makes the dump restorable over an existing database.
	if pg_dump -h postgres -U happyfeet -d happyfeet --clean --if-exists \
		| gzip -9 > "$file.tmp"; then
		mv "$file.tmp" "$file"
		log "wrote $(du -h "$file" | cut -f1)"
	else
		rm -f "$file.tmp"
		log "ERROR: pg_dump failed"
		return 1
	fi

	# Verify the archive is intact before trusting it or deleting older ones.
	if ! gzip -t "$file"; then
		log "ERROR: $file is corrupt, removing"
		rm -f "$file"
		return 1
	fi

	if [ -n "${S3_BACKUP_BUCKET:-}" ]; then
		if command -v aws >/dev/null 2>&1; then
			log "uploading to s3://$S3_BACKUP_BUCKET/"
			aws s3 cp "$file" "s3://$S3_BACKUP_BUCKET/" \
				${S3_BACKUP_ENDPOINT:+--endpoint-url "$S3_BACKUP_ENDPOINT"} \
				|| log "WARNING: upload failed, local copy retained"
		else
			log "WARNING: S3_BACKUP_BUCKET set but the aws CLI is not installed"
		fi
	fi

	log "pruning dumps older than $KEEP_DAYS days"
	find "$BACKUP_DIR" -name 'happyfeet-*.sql.gz' -mtime "+$KEEP_DAYS" -delete
	log "done — $(ls -1 "$BACKUP_DIR"/happyfeet-*.sql.gz 2>/dev/null | wc -l) archives retained"
}

log "backup loop started (every ${INTERVAL}s, keeping ${KEEP_DAYS} days)"
while true; do
	run_backup || log "backup cycle failed; will retry next interval"
	sleep "$INTERVAL"
done
