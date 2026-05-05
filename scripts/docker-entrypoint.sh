#!/bin/sh
# Ensure /data is writable by the vocabgen user.
# Runs as root, then drops to vocabgen via exec su-exec.
chown -R vocabgen:vocabgen /data 2>/dev/null || true
exec su-exec vocabgen "$@"
