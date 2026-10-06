package main

import (
	"encoding/json"
	"flag"
	"fmt"
	sdk "iot-smart-system/sdk"
	"log"
	"net/http"
	"sort"
	"sync"
)

// ServiceInfo описывает зарегистрированную в системе подпрограмму
type ServiceInfo struct {
	ID    string `json:"id"`    // Уникальный ключ (например: "go_weather")
	Name  string `json:"name"`  // Красивое имя для вывода в меню на ESP32 (например: "WEATHER STATION")
	Topic string `json:"topic"` // MQTT-топик, куда эта подпрограмма льет кадры (например: "orange/display/weather")
}

var (
	// Хранилище активных топиков/подпрограмм в памяти (Thread-safe)
	registry = make(map[string]ServiceInfo)
	mu       sync.RWMutex
)

func main() {
	// Флаг для смены порта при запуске (по умолчанию 8080)
	port := flag.Int("port", 8080, "Порт для HTTP сервера Service Discovery")
	flag.Parse()

	// Настраиваем ручки (endpoints)
	http.HandleFunc(string(sdk.RouteRegister), handleRegister)     // Для регистрации подпрограмм (POST)
	http.HandleFunc(string(sdk.RouteUnregister), handleUnregister) // Для удаления подпрограмм (POST)
	http.HandleFunc(string(sdk.RouteTopics), handleGetTopics)      // Для ESP32 (GET запрос списка)

	log.Printf("[REGISTRY] Центральный диспетчер запущен на порту :%d\n", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		log.Fatalf("Критическая ошибка сервера: %v", err)
	}
}

// POST /register — подпрограмма (Go/Java) сообщает о себе при старте
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешен только POST метод", http.StatusMethodNotAllowed)
		return
	}

	var svc ServiceInfo
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		http.Error(w, "Кривой JSON пакета", http.StatusBadRequest)
		return
	}

	if svc.ID == "" || svc.Name == "" || svc.Topic == "" {
		http.Error(w, "Поля ID, Name и Topic не могут быть пустыми", http.StatusBadRequest)
		return
	}

	mu.Lock()
	registry[svc.ID] = svc
	mu.Unlock()

	log.Printf("[REGISTRY] Подпрограмма зарегала топик: [%s] '%s' -> топик: %s\n", svc.ID, svc.Name, svc.Topic)
	w.WriteHeader(http.StatusOK)
}

// POST /unregister — подпрограмма корректно завершает работу и удаляет себя из меню
func handleUnregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Разрешен только POST метод", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Кривой JSON", http.StatusBadRequest)
		return
	}

	mu.Lock()
	delete(registry, req.ID)
	mu.Unlock()

	log.Printf("[REGISTRY] Сервис с ID [%s] удален из реестра\n", req.ID)
	w.WriteHeader(http.StatusOK)
}

// GET /topics — ESP32 забирает массив всех доступных экранов
func handleGetTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Разрешен только GET метод", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	mu.RLock()
	list := make([]ServiceInfo, 0, len(registry))
	for _, svc := range registry {
		list = append(list, svc)
	}
	mu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })

	// Отправляем массив структур в JSON формате
	json.NewEncoder(w).Encode(list)
}
