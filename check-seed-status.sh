#!/bin/bash

# Seed Node Status Check Script
# Prüft den Status der Seed Node und zeigt wichtige Informationen

echo "════════════════════════════════════════════════════════════"
echo "🔍 SEED NODE STATUS CHECK"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. Service-Status
echo "1️⃣ Service-Status:"
echo "────────────────────────────────────────────────────────────"
sudo systemctl status kalon-seed-node --no-pager | head -20
echo ""

# 2. Prozess-Status
echo "2️⃣ Prozess-Status:"
echo "────────────────────────────────────────────────────────────"
if ps aux | grep -v grep | grep kalon-node-v2 > /dev/null; then
    ps aux | grep -v grep | grep kalon-node-v2
    echo "✅ Node-Prozess läuft"
else
    echo "❌ Node-Prozess läuft NICHT"
fi
echo ""

# 3. Block-Höhe
echo "3️⃣ Block-Höhe:"
echo "────────────────────────────────────────────────────────────"
HEIGHT=$(curl -s -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getHeight","params":{},"id":1}' 2>/dev/null | jq -r '.result // "ERROR"' 2>/dev/null || echo "ERROR")

if [ "$HEIGHT" != "ERROR" ] && [ "$HEIGHT" != "null" ]; then
    echo "✅ Aktuelle Block-Höhe: $HEIGHT"
else
    echo "❌ Konnte Block-Höhe nicht abrufen (Node läuft möglicherweise nicht oder RPC nicht erreichbar)"
fi
echo ""

# 4. Peer-Count
echo "4️⃣ Verbundene Peers:"
echo "────────────────────────────────────────────────────────────"
PEER_COUNT=$(curl -s -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getPeerCount","params":{},"id":1}' 2>/dev/null | jq -r '.result // "ERROR"' 2>/dev/null || echo "ERROR")

if [ "$PEER_COUNT" != "ERROR" ] && [ "$PEER_COUNT" != "null" ]; then
    echo "✅ Verbundene Peers: $PEER_COUNT"
else
    echo "⚠️  Konnte Peer-Count nicht abrufen"
fi
echo ""

# 5. Letzte Log-Zeilen
echo "5️⃣ Letzte Log-Zeilen (aus logs/node.log):"
echo "────────────────────────────────────────────────────────────"
if [ -f "logs/node.log" ]; then
    tail -10 logs/node.log
else
    echo "⚠️  Log-Datei nicht gefunden: logs/node.log"
    echo "💡 Versuche journalctl..."
    sudo journalctl -u kalon-seed-node -n 10 --no-pager
fi
echo ""

# 6. Port-Status
echo "6️⃣ Port-Status:"
echo "────────────────────────────────────────────────────────────"
if command -v ss &> /dev/null; then
    echo "RPC Port (16316):"
    ss -tlnp | grep 16316 || echo "  ⚠️  Port 16316 nicht in Verwendung"
    echo ""
    echo "P2P Port (17335):"
    ss -tlnp | grep 17335 || echo "  ⚠️  Port 17335 nicht in Verwendung"
else
    echo "⚠️  'ss' Befehl nicht verfügbar, kann Ports nicht prüfen"
fi
echo ""

echo "════════════════════════════════════════════════════════════"
echo "✅ Status-Check abgeschlossen"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "💡 Nützliche Befehle:"
echo "   - Logs live ansehen: tail -f logs/node.log"
echo "   - Oder mit journalctl: sudo journalctl -u kalon-seed-node -f"
echo "   - Node neustarten: sudo systemctl restart kalon-seed-node"
echo "   - Node stoppen: sudo systemctl stop kalon-seed-node"
echo "   - Node starten: sudo systemctl start kalon-seed-node"
echo ""

