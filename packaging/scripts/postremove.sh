#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload || true
fi

# We intentionally keep the pmox user and /etc/pmox (config + registry data) on
# uninstall so an accidental removal does not destroy approvals/licenses.
# Remove them manually for a full purge:
#   userdel pmox; rm -rf /etc/pmox

exit 0
