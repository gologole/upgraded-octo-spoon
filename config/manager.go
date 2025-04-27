package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigManager управляет конфигурацией и поддерживает горячую перезагрузку
type ConfigManager struct {
	mu          sync.RWMutex
	config      *Config
	configPath  string
	subscribers []chan<- *Config
	lastError   error
	watcher     *fsnotify.Watcher
}

// NewConfigManager создает новый менеджер конфигурации
func NewConfigManager(configPath string) (*ConfigManager, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	manager := &ConfigManager{
		configPath:  configPath,
		subscribers: make([]chan<- *Config, 0),
		watcher:     watcher,
	}

	// Загружаем начальную конфигурацию
	if err := manager.loadConfig(); err != nil {
		return nil, err
	}

	// Запускаем отслеживание изменений
	go manager.watchConfig()

	return manager, nil
}

// Subscribe подписывает на изменения конфигурации
func (m *ConfigManager) Subscribe() <-chan *Config {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *Config, 1)
	m.subscribers = append(m.subscribers, ch)

	// Сразу отправляем текущую конфигурацию
	if m.config != nil {
		ch <- m.config
	}

	return ch
}

// GetConfig возвращает текущую конфигурацию
func (m *ConfigManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetLastError возвращает последнюю ошибку загрузки конфигурации
func (m *ConfigManager) GetLastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

// Close закрывает менеджер и освобождает ресурсы
func (m *ConfigManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Закрываем все подписки
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil

	return m.watcher.Close()
}

// loadConfig загружает конфигурацию из файла
func (m *ConfigManager) loadConfig() error {
	newConfig, err := LoadFromFile(m.configPath)
	if err != nil {
		m.mu.Lock()
		m.lastError = err
		m.mu.Unlock()
		return fmt.Errorf("failed to load config: %w", err)
	}

	m.mu.Lock()
	m.config = newConfig
	m.lastError = nil

	// Уведомляем подписчиков
	for _, ch := range m.subscribers {
		select {
		case ch <- newConfig:
		default:
			// Если канал заполнен, пропускаем
		}
	}
	m.mu.Unlock()

	return nil
}

// watchConfig отслеживает изменения в файле конфигурации
func (m *ConfigManager) watchConfig() {
	// Добавляем файл для отслеживания
	if err := m.watcher.Add(m.configPath); err != nil {
		m.mu.Lock()
		m.lastError = fmt.Errorf("failed to watch config file: %w", err)
		m.mu.Unlock()
		return
	}

	// Добавляем дебаунс для множественных событий
	var debounceTimer *time.Timer
	const debounceDelay = 100 * time.Millisecond

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Сбрасываем таймер если он уже запущен
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// Запускаем новый таймер
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					m.loadConfig()
				})
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.mu.Lock()
			m.lastError = fmt.Errorf("watcher error: %w", err)
			m.mu.Unlock()
		}
	}
}
