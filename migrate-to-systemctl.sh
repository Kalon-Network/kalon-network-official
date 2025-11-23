#!/bin/bash

# Kalon Network - Migrate to systemctl
# Stoppt alle laufenden Node-Prozesse und startet über systemctl

set -e

echo "════════════════════════════════════════════════════════════"
echo "🔄 MIGRATION ZU SYSTEMCTL"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. Stoppe alle laufenden Node-Prozesse
echo "1️⃣  Stoppe alle laufenden Node-Prozesse..."
if pgrep -f "kalon-node-v2" > /dev/null; then
    echo "   Gefundene Prozesse:"
    ps aux | grep -v grep | grep kalon-node-v2
    echo ""
    echo "   Stoppe Prozesse..."
    pkill -f kalon-node-v2
    sleep 3
    
    # Falls noch Prozesse laufen, force kill
    if pgrep -f "kalon-node-v2" > /dev/null; then
        echo "   Force kill..."
        pkill -9 -f kalon-node-v2
        sleep 2
    fi
    
    echo "✅ Alle Node-Prozesse gestoppt"
else
    echo "✅ Keine laufenden Node-Prozesse gefunden"
fi
echo ""

# 2. Prüfe ob systemd Service existiert
echo "2️⃣  Prüfe systemd Service..."
if [ -f "/etc/systemd/system/kalon-seed-node.service" ]; then
    echo "✅ Service existiert bereits"
else
    echo "⚠️  Service existiert nicht, erstelle ihn..."
    
    # Erstelle Service-Datei
    sudo tee /etc/systemd/system/kalon-seed-node.service > /dev/null <<EOF
[Unit]
Description=Kalon Network Seed Node
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/build/kalon-node-v2 \\
    -datadir $(pwd)/data/testnet \\
    -genesis $(pwd)/genesis/testnet.json \\
    -rpc 127.0.0.1:16316 \\
    -p2p 0.0.0.0:17335 \\
    -seednodes 185.133.249.107:17335
Restart=always
RestartSec=10
StandardOutput=append:$(pwd)/logs/node.log
StandardError=append:$(pwd)/logs/node.log

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    echo "✅ Service erstellt"
fi
echo ""

# 3. Prüfe ob Binary existiert
echo "3️⃣  Prüfe Binary..."
if [ ! -f "build/kalon-node-v2" ]; then
    echo "❌ Binary nicht gefunden: build/kalon-node-v2"
    echo "   Baue Binary..."
    go build -o build/kalon-node-v2 cmd/kalon-node-v2/main.go
    if [ $? -eq 0 ]; then
        echo "✅ Binary gebaut"
    else
        echo "❌ Build fehlgeschlagen"
        exit 1
    fi
else
    echo "✅ Binary existiert"
fi
echo ""

# 4. Erstelle notwendige Verzeichnisse
echo "4️⃣  Erstelle Verzeichnisse..."
mkdir -p logs data/testnet/chaindb
echo "✅ Verzeichnisse erstellt"
echo ""

# 5. Starte Node über systemctl
echo "5️⃣  Starte Node über systemctl..."
sudo systemctl enable kalon-seed-node
sudo systemctl start kalon-seed-node

# Warte kurz
sleep 5

# Prüfe Status
if sudo systemctl is-active --quiet kalon-seed-node; then
    echo "✅ Node läuft über systemctl"
else
    echo "❌ Node konnte nicht gestartet werden"
    echo ""
    echo "Prüfe Logs:"
    sudo journalctl -u kalon-seed-node -n 50 --no-pager
    exit 1
fi
echo ""

# 6. Prüfe ob Node antwortet
echo "6️⃣  Prüfe ob Node antwortet..."
sleep 5
HEIGHT=$(curl -s -X POST http://localhost:16316/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"getHeight","params":{},"id":1}' 2>/dev/null | jq -r '.result // "ERROR"' 2>/dev/null || echo "ERROR")

if [ "$HEIGHT" != "ERROR" ] && [ "$HEIGHT" != "null" ]; then
    echo "✅ Node antwortet - Block-Höhe: $HEIGHT"
else
    echo "⚠️  Node antwortet noch nicht (kann einige Sekunden dauern)"
fi
echo ""

# 7. Zeige Status
echo "7️⃣  Service-Status:"
echo "────────────────────────────────────────────────────────────"
sudo systemctl status kalon-seed-node --no-pager | head -20
echo ""

echo "════════════════════════════════════════════════════════════"
echo "✅ MIGRATION ABGESCHLOSSEN!"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "📋 Nützliche Befehle:"
echo ""
echo "   Status prüfen:"
echo "   sudo systemctl status kalon-seed-node"
echo ""
echo "   Logs ansehen:"
echo "   tail -f logs/node.log"
echo "   Oder: sudo journalctl -u kalon-seed-node -f"
echo ""
echo "   Node neustarten:"
echo "   sudo systemctl restart kalon-seed-node"
echo ""
echo "   Node stoppen:"
echo "   sudo systemctl stop kalon-seed-node"
echo ""
echo "   Node starten:"
echo "   sudo systemctl start kalon-seed-node"
echo ""

