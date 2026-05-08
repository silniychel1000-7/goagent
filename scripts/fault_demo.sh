#!/usr/bin/env bash
# =============================================================================
# fault_demo.sh — Demonstration script for diploma defense
#
# Scenario:
#   1. Show current metric state
#   2. Bring down link R1-R2 (simulate failure)
#   3. Wait for alert to fire in Prometheus
#   4. Restore the link
#
# Usage:
#   ./fault_demo.sh [--target clab-netmon-r1] [--iface eth1]
# =============================================================================
set -euo pipefail

TARGET_NODE="${TARGET_NODE:-clab-netmon-r1}"
IFACE="${IFACE:-eth1}"
WAIT_ALERT_SEC="${WAIT_ALERT_SEC:-90}"
PROM_URL="${PROM_URL:-http://localhost:9090}"

RED='\033[0;31m'
GRN='\033[0;32m'
YLW='\033[1;33m'
CYN='\033[0;36m'
NC='\033[0m'

step() { echo -e "\n${CYN}[STEP]${NC} $*"; }
ok()   { echo -e "${GRN}[OK]${NC}   $*"; }
warn() { echo -e "${YLW}[WAIT]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }

# ---------------------------------------------------------------------------
step "1/5  Current network state"
# ---------------------------------------------------------------------------
echo "Active Prometheus alerts:"
curl -sf "${PROM_URL}/api/v1/alerts" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
alerts = d.get('data',{}).get('alerts',[])
if not alerts:
    print('  (none — all healthy)')
for a in alerts:
    print(f\"  [{a['labels'].get('severity','?').upper()}] {a['labels'].get('alertname')} — {a['labels'].get('target','')}\")
" 2>/dev/null || echo "  (could not reach Prometheus at ${PROM_URL})"

echo ""
echo "Current latency snapshot (network_latency_ms):"
curl -sf "${PROM_URL}/api/v1/query?query=network_latency_ms" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('data',{}).get('result',[]):
    t = r['metric'].get('target','?')
    v = r['value'][1]
    print(f'  {t}: {float(v):.1f} ms')
" 2>/dev/null || echo "  (could not reach Prometheus)"

# ---------------------------------------------------------------------------
step "2/5  Injecting fault: bringing down ${TARGET_NODE} / ${IFACE}"
# ---------------------------------------------------------------------------
echo "Command: docker exec ${TARGET_NODE} ip link set ${IFACE} down"
docker exec "${TARGET_NODE}" ip link set "${IFACE}" down
ok "Interface ${IFACE} is DOWN on ${TARGET_NODE}"

# ---------------------------------------------------------------------------
step "3/5  Waiting ${WAIT_ALERT_SEC}s for HostDown alert to fire..."
# ---------------------------------------------------------------------------
warn "Check Grafana at http://localhost:3000 — you should see packet loss spike"
warn "Check Alertmanager at http://localhost:9093"

for i in $(seq 1 "${WAIT_ALERT_SEC}"); do
    sleep 1
    firing=$(curl -sf "${PROM_URL}/api/v1/alerts" \
        | python3 -c "
import sys,json
d=json.load(sys.stdin)
print(len([a for a in d.get('data',{}).get('alerts',[]) if a.get('state')=='firing']))
" 2>/dev/null || echo "0")

    if [ "${firing}" -gt 0 ]; then
        ok "Alert fired after ${i}s! Firing alerts:"
        curl -sf "${PROM_URL}/api/v1/alerts" \
          | python3 -c "
import sys, json
d = json.load(sys.stdin)
for a in d.get('data',{}).get('alerts',[]):
    if a.get('state') == 'firing':
        print(f\"  [{a['labels'].get('severity','?').upper()}] {a['labels'].get('alertname')} — {a['labels'].get('target','')}\")
"
        break
    fi

    printf "\r  ${i}/${WAIT_ALERT_SEC}s elapsed, waiting for alert..."
done
echo ""

# ---------------------------------------------------------------------------
step "4/5  Restoring link: bringing ${IFACE} back UP"
# ---------------------------------------------------------------------------
docker exec "${TARGET_NODE}" ip link set "${IFACE}" up
ok "Interface ${IFACE} is UP on ${TARGET_NODE}"

# ---------------------------------------------------------------------------
step "5/5  Post-recovery check (30s)"
# ---------------------------------------------------------------------------
sleep 30
echo "Latency after recovery:"
curl -sf "${PROM_URL}/api/v1/query?query=network_latency_ms" \
  | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d.get('data',{}).get('result',[]):
    t = r['metric'].get('target','?')
    v = r['value'][1]
    print(f'  {t}: {float(v):.1f} ms')
" 2>/dev/null || echo "  (could not reach Prometheus)"

ok "Demo complete. Check Telegram for alert + resolution messages."
