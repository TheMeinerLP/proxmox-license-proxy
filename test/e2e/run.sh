#!/usr/bin/env bash
# End-to-end test: drive the full subscription lifecycle across two containers
# (proxy + simulated Proxmox host) with no real Proxmox install. Catches
# regressions in serve, client enroll, account approval, key issuance/set,
# socket licensing, listing and revocation.
set -euo pipefail
cd "$(dirname "$0")"

PROXY_URL="https://proxy:8443"
ADMIN_TOKEN="e2e-secret"
CEXEC="docker compose exec -T client"
PEXEC="docker compose exec -T proxy"

pass() { echo "  ok: $*"; }
fail() {
	echo "FAIL: $*" >&2
	echo "----- proxy logs -----" >&2
	docker compose logs proxy >&2 2>&1 || true
	exit 1
}
cleanup() { docker compose down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

echo "==> building the binary the images embed"
mkdir -p bin
(cd ../.. && CGO_ENABLED=0 GOOS=linux go build -o test/e2e/bin/proxmox-license-proxy .)

echo "==> starting proxy + client (proxy waited healthy)"
docker compose up -d --build

# Belt-and-suspenders: confirm the proxy answers readyz from the client's network.
for i in $(seq 1 30); do
	if $CEXEC curl -fsS -k "$PROXY_URL/readyz" >/dev/null 2>&1; then break; fi
	[ "$i" = 30 ] && fail "proxy never became ready"
	sleep 1
done
pass "proxy is ready"

echo "==> enroll #1 (expect the account to be PENDING)"
out=$($CEXEC proxmox-license-proxy client enroll \
	--server "$PROXY_URL" --no-redirect --products pve,pbs --sockets 2 2>&1 || true)
echo "$out" | sed 's/^/    /'
echo "$out" | grep -q "not approved" || fail "first enroll should report the account pending"
thumb=$(printf '%s\n' "$out" | sed -n 's/^account: \([A-Za-z0-9_-]*\).*/\1/p')
[ -n "$thumb" ] || fail "could not parse the account thumbprint from enroll output"
pass "account is pending: $thumb"

echo "==> approve the account on the proxy"
$PEXEC proxmox-license-proxy account approve "$thumb" | sed 's/^/    /'
pass "account approved"

echo "==> enroll #2 (expect keys issued and set)"
out=$($CEXEC proxmox-license-proxy client enroll \
	--server "$PROXY_URL" --no-redirect --products pve,pbs --sockets 2 2>&1)
echo "$out" | sed 's/^/    /'
echo "$out" | grep -q "issued pve" || fail "pve key was not issued"
echo "$out" | grep -q "issued pbs" || fail "pbs key was not issued"
echo "$out" | grep -q "installed on Proxmox VE" || fail "pve key was not set on the host"
echo "$out" | grep -q "enroll complete" || fail "enroll did not complete"
pass "both products enrolled and set"

echo "==> the fake tools recorded the keys (PVE at the 2-socket tier)"
pvekey=$($CEXEC sh -c 'cat /var/lib/fake-proxmox/pve.key' | tr -d '\r')
pbskey=$($CEXEC sh -c 'cat /var/lib/fake-proxmox/pbs.key' | tr -d '\r')
echo "$pvekey" | grep -qE '^pve2[cbsp]-[0-9a-f]{10}$' || fail "pve key is not a valid 2-socket key: '$pvekey'"
echo "$pbskey" | grep -qE '^pbs[cbsp]-[0-9a-f]{10}$' || fail "pbs key is malformed: '$pbskey'"
pass "pve=$pvekey pbs=$pbskey"

echo "==> the proxy lists both subscriptions as active"
list=$($PEXEC env NO_COLOR=1 proxmox-license-proxy subscription list)
echo "$list" | sed 's/^/    /'
echo "$list" | grep -- "$pvekey" | grep -q APPROVED || fail "pve subscription is not active on the proxy"
echo "$list" | grep -- "$pbskey" | grep -q APPROVED || fail "pbs subscription is not active on the proxy"
pass "both subscriptions active"

echo "==> verify.php honours the issued key (status active)"
vout=$($CEXEC curl -fsS -k "$PROXY_URL/modules/servers/licensing/verify.php" \
	--data-urlencode "licensekey=$pvekey" \
	--data-urlencode "dir=e2e-host" \
	--data-urlencode "check_token=tok123")
echo "$vout" | grep -q "<status>active</status>" || fail "verify.php did not report active: $vout"
pass "verify.php active"

echo "==> revoke the pve subscription via the admin API"
$CEXEC curl -fsS -k -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
	"$PROXY_URL/api/v1/admin/subscriptions/$pvekey/revoke" >/dev/null
list=$($PEXEC env NO_COLOR=1 proxmox-license-proxy subscription list)
echo "$list" | grep -- "$pvekey" | grep -q REVOKED || fail "pve subscription was not revoked"
pass "pve subscription revoked"

echo "==> verify.php now rejects the revoked key"
vout=$($CEXEC curl -fsS -k "$PROXY_URL/modules/servers/licensing/verify.php" \
	--data-urlencode "licensekey=$pvekey" \
	--data-urlencode "dir=e2e-host" \
	--data-urlencode "check_token=tok123")
if echo "$vout" | grep -q "<status>active</status>"; then
	fail "verify.php still reports active after revoke: $vout"
fi
pass "verify.php no longer active for the revoked key"

echo
echo "ALL E2E CHECKS PASSED"
