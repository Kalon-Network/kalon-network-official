#!/bin/bash

# Kalon Network - Quick Seed Node Check
# Schneller Check für Seed Node Status

echo "=========================================="
echo "🔍 QUICK SEED NODE CHECK"
echo "=========================================="
echo ""

# 1. Service Status
echo "1️⃣  SERVICE STATUS:"
echo "----------------------------------------"
sudo systemctl status kalon-seed-node --no-pager | head -20
echo ""

# 2. Logs prüfen
echo "2️⃣  LETZTE LOGS (journalctl):"
echo "----------------------------------------"
sudo journalctl -u kalon-seed-node -n 100 --no-pager | tail -30
echo ""

# 3. RPC Port prüfen
echo "3️⃣  RPC PORT (16316):"
echo "----------------------------------------"
if command -v ss > /dev/null; then
    if sudo ss -tulpn | grep 16316 > /dev/null; then
        echo "✅ Port 16316 ist offen:"
        sudo ss -tulpn | grep 16316
    else
        echo "❌ Port 16316 ist NICHT offen!"
    fi
elif command -v netstat > /dev/null; then
    if sudo netstat -tulpn | grep 16316 > /dev/null; then
        echo "✅ Port 16316 ist offen:"
        sudo netstat -tulpn | grep 16316
    else
        echo "❌ Port 16316 ist NICHT offen!"
    fi
else
    echo "⚠️  ss und netstat nicht verfügbar - Port-Check übersprungen"
fi
echo ""

# 4. P2P Port prüfen
echo "4️⃣  P2P PORT (17335):"
echo "----------------------------------------"
if command -v ss > /dev/null; then
    if sudo ss -tulpn | grep 17335 > /dev/null; then
        echo "✅ Port 17335 ist offen:"
        sudo ss -tulpn | grep 17335
    else
        echo "❌ Port 17335 ist NICHT offen!"
    fi
elif command -v netstat > /dev/null; then
    if sudo netstat -tulpn | grep 17335 > /dev/null; then
        echo "✅ Port 17335 ist offen:"
        sudo netstat -tulpn | grep 17335
    else
        echo "❌ Port 17335 ist NICHT offen!"
    fi
else
    echo "⚠️  ss und netstat nicht verfügbar - Port-Check übersprungen"
fi
echo ""

# 5. Prozess prüfen
echo "5️⃣  PROZESS PRÜFEN:"
echo "----------------------------------------"
if ps aux | grep -v grep | grep kalon-node-v2 > /dev/null; then
    echo "✅ Node-Prozess läuft:"
    ps aux | grep -v grep | grep kalon-node-v2
else
    echo "❌ Node-Prozess läuft NICHT!"
fi
echo ""

# 6. RPC Test
echo "6️⃣  RPC SERVER TEST:"
echo "----------------------------------------"
RPC_RESPONSE=$(curl -s -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getHeight","params":{},"id":1}' 2>&1)

if [ $? -eq 0 ] && [ ! -z "$RPC_RESPONSE" ]; then
    echo "✅ RPC Server antwortet:"
    echo "$RPC_RESPONSE" | jq '.' 2>/dev/null || echo "$RPC_RESPONSE"
    HEIGHT=$(echo "$RPC_RESPONSE" | jq -r '.result' 2>/dev/null)
    if [ ! -z "$HEIGHT" ] && [ "$HEIGHT" != "null" ]; then
        echo ""
        echo "📊 Blockhöhe: $HEIGHT"
    fi
else
    echo "❌ RPC Server antwortet NICHT!"
    echo "Fehler: $RPC_RESPONSE"
    echo ""
    echo "🔍 Mögliche Ursachen:"
    echo "   - RPC Server wurde nicht gestartet"
    echo "   - Port 16316 ist nicht offen"
    echo "   - Service läuft nicht korrekt"
    echo ""
    echo "💡 Prüfe die Logs oben für Fehler!"
fi
echo ""

# 7. Log-Datei prüfen
echo "7️⃣  LOG-DATEI:"
echo "----------------------------------------"
LOG_FILE="logs/node.log"
if [ -f "$LOG_FILE" ]; then
    echo "✅ Log-Datei existiert: $LOG_FILE"
    echo "Letzte 10 Zeilen:"
    tail -10 "$LOG_FILE"
else
    echo "⚠️  Log-Datei existiert NICHT: $LOG_FILE"
    echo "   (Logs werden nur in systemd journal geschrieben)"
    echo ""
    echo "💡 Verwende: sudo journalctl -u kalon-seed-node -f"
fi
echo ""

echo "=========================================="
echo "📋 ZUSAMMENFASSUNG:"
echo "=========================================="

# Prüfe kritische Punkte
ERRORS=0

if ! sudo systemctl is-active --quiet kalon-seed-node; then
    echo "❌ Service läuft NICHT"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ Service läuft"
fi

if ! ps aux | grep -v grep | grep kalon-node-v2 > /dev/null; then
    echo "❌ Node-Prozess läuft NICHT"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ Node-Prozess läuft"
fi

# Port-Check mit ss oder netstat
PORT_CHECK_FAILED=false
if command -v ss > /dev/null; then
    if ! sudo ss -tulpn | grep -q 16316; then
        PORT_CHECK_FAILED=true
    fi
elif command -v netstat > /dev/null; then
    if ! sudo netstat -tulpn | grep -q 16316; then
        PORT_CHECK_FAILED=true
    fi
fi

if [ "$PORT_CHECK_FAILED" = true ]; then
    echo "❌ RPC Port (16316) ist NICHT offen"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ RPC Port (16316) ist offen"
fi

PORT_CHECK_FAILED=false
if command -v ss > /dev/null; then
    if ! sudo ss -tulpn | grep -q 17335; then
        PORT_CHECK_FAILED=true
    fi
elif command -v netstat > /dev/null; then
    if ! sudo netstat -tulpn | grep -q 17335; then
        PORT_CHECK_FAILED=true
    fi
fi

if [ "$PORT_CHECK_FAILED" = true ]; then
    echo "❌ P2P Port (17335) ist NICHT offen"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ P2P Port (17335) ist offen"
fi

if [ -z "$HEIGHT" ] || [ "$HEIGHT" = "null" ]; then
    echo "❌ RPC Server antwortet NICHT"
    ERRORS=$((ERRORS + 1))
else
    echo "✅ RPC Server funktioniert (Blockhöhe: $HEIGHT)"
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "✅ ALLES OK - Seed Node läuft korrekt!"
else
    echo "⚠️  $ERRORS Problem(e) gefunden!"
    echo ""
    echo "🔧 NÄCHSTE SCHRITTE:"
    echo "   1. Prüfe die Logs oben für Fehler"
    echo "   2. Prüfe Service-Status: sudo systemctl status kalon-seed-node"
    echo "   3. Prüfe Live-Logs: sudo journalctl -u kalon-seed-node -f"
    echo "   4. Restart Service: sudo systemctl restart kalon-seed-node"
fi
echo ""

