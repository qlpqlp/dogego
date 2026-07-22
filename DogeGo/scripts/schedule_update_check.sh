#!/usr/bin/env bash
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT
# Print a cron line or run the GitHub release check once.
set -euo pipefail
cd "$(dirname "$0")/.."
SCRIPT="$(cd "$(dirname "$0")" && pwd)/check_update.sh"
if [[ "${1:-}" == "--install-cron" ]]; then
  echo "0 9 * * * cd $(pwd) && $SCRIPT >> ${DOGEGO_UPDATE_CRON_LOG:-$HOME/dogego-update-check.log} 2>&1"
  exit 0
fi
exec "$SCRIPT" "$@"
