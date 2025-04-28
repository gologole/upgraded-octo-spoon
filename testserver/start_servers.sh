#!/bin/bash

# Собираем сервер
go build -o echo_server main.go

# Запускаем несколько экземпляров с разными портами и сообщениями
./echo_server -port 8081 -message "Server 1: Primary Backend" &
./echo_server -port 8082 -message "Server 2: Secondary Backend" &
./echo_server -port 8083 -message "Server 3: Tertiary Backend" &

# Ждем нажатия Ctrl+C
echo "Servers started. Press Ctrl+C to stop all servers."
wait

# При получении Ctrl+C останавливаем все процессы
trap 'kill $(jobs -p)' EXIT 