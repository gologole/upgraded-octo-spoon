package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
)

var (
	port    = flag.String("port", "8080", "порт для прослушивания")
	message = flag.String("message", "Hello from test server", "сообщение для ответа")
)

func main() {
	flag.Parse()

	// Настраиваем логгер
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix(fmt.Sprintf("[Echo Server :%s] ", *port))

	// Регистрируем обработчики
	http.HandleFunc("/", handleRequest)
	http.HandleFunc("/health", handleHealth)

	// Запускаем сервер
	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Starting server on %s with message: %s", addr, *message)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Логируем входящий запрос
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Printf("Error dumping request: %v", err)
	} else {
		log.Printf("Incoming request:\n%s", string(dump))
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Формируем ответ
	response := fmt.Sprintf("Server Message: %s\n\nRequest Details:\nMethod: %s\nPath: %s\nHeaders:\n",
		*message, r.Method, r.URL.Path)

	// Добавляем заголовки
	for name, values := range r.Header {
		for _, value := range values {
			response += fmt.Sprintf("%s: %s\n", name, value)
		}
	}

	// Добавляем тело запроса, если оно есть
	if len(body) > 0 {
		response += fmt.Sprintf("\nRequest Body:\n%s", string(body))
	}

	// Отправляем ответ
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, response)

	// Логируем ответ
	log.Printf("Sent response:\n%s", response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status": "ok"}`)
}
