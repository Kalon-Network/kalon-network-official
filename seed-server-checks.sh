#!/bin/bash

# Kalon Network - Seed Server Checks
# Führe diese Befehle auf dem Seed Server (158.220.100.245) aus

echo "=========================================="
echo "🔍 SEED SERVER DIAGNOSE"
echo "=========================================="
echo ""

# 1. Service Status
echo "1️⃣  SERVICE STATUS:"
echo "----------------------------------------"
sudo systemctl status kalon-seed-node
echo ""

# 2. Logs prüfen
echo "2️⃣  LOGS (journalctl - letzte 100 Zeilen):"
echo "----------------------------------------"
sudo journalctl -u kalon-seed-node -n 100 --no-pager
echo ""

# 3. RPC Port prüfen
echo "3️⃣  RPC PORT (16316):"
echo "----------------------------------------"
sudo netstat -tulpn | grep 16316
echo ""

# 4. P2P Port prüfen
echo "4️⃣  P2P PORT (17335):"
echo "----------------------------------------"
sudo netstat -tulpn | grep 17335
echo ""

# 5. Prozess prüfen
echo "5️⃣  PROZESS PRÜFEN:"
echo "----------------------------------------"
ps aux | grep kalon-node-v2
echo ""

# 6. RPC Test
echo "6️⃣  RPC SERVER TEST:"
echo "----------------------------------------"
curl -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getHeight","params":{},"id":1}'
echo ""
echo ""

# 7. Peer Count Test
echo "7️⃣  PEER COUNT TEST:"
echo "----------------------------------------"
curl -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getPeerCount","params":{},"id":1}'
echo ""
echo ""

echo "=========================================="
echo "✅ DIAGNOSE ABGESCHLOSSEN"
echo "=========================================="

