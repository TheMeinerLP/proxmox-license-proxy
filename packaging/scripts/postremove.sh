#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

# We intentionally keep the pmox user and /var/lib/pmox (registry data) on
# uninstall so an accidental removal does not destroy approvals/licenses.
# Remove them manually for a full purge:
#   userdel pmox; rm -rf /var/lib/pmox

exit 0
