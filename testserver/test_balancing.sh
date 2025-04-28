#!/bin/bash

# Цвета для вывода
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для отправки запроса и форматированного вывода ответа
send_request() {
    local response
    response=$(curl -s "http://localhost:8080/test")
    local server_msg=$(echo "$response" | grep "Server Message:" | cut -d':' -f2-)
    echo -e "${BLUE}Request $1:${NC} ${GREEN}$server_msg${NC}"
}

echo "Starting load balancing test..."
echo "Sending 10 requests to proxy (localhost:8080)..."
echo "================================================"

# Отправляем 10 запросов
for i in {1..10}; do
    send_request $i
    sleep 0.5 # Небольшая задержка между запросами
done

echo "================================================"
echo "Test completed. В случае Round Robin балансировки вы должны видеть"
echo "последовательное распределение запросов между серверами 1, 2 и 3" 