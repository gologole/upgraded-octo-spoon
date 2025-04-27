package app

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"cloud.ru_test/internal/loadbalancer"

	"cloud.ru_test/config"
	"cloud.ru_test/internal/ratelimit"
	"cloud.ru_test/internal/transport"
	"cloud.ru_test/pkg/logger"
)

type App struct {
	configManager *config.ConfigManager
	proxy         *transport.Proxy
	appLogger     *logger.CustomZapLogger
	mu            sync.Mutex
	port          string
}

func NewApp(configPath, port string) (*App, error) {
	// Создаем менеджер конфигурации
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	app := &App{
		configManager: configManager,
		port:          port,
	}

	// Создаем логгер
	app.appLogger = logger.NewCustomZapLogger((*logger.LoggerConfig)(configManager.GetConfig().Logger))
	app.appLogger.Info(fmt.Sprintf("Инициализация приложения (configPath: %s, port: %s)", configPath, port))

	// Подписываемся на изменения конфигурации
	configCh := configManager.Subscribe()
	go app.watchConfig(configCh)
	app.appLogger.Info("Запущено отслеживание изменений конфигурации")

	return app, nil
}

func (a *App) watchConfig(configCh <-chan *config.Config) {
	for cfg := range configCh {
		a.appLogger.Info(fmt.Sprintf("Получена новая конфигурация (метод балансировки: %s)", cfg.LoadBalancer.Method))
		if err := a.reconfigure(cfg); err != nil {
			a.appLogger.Error(fmt.Sprintf("Ошибка при реконфигурации приложения: %v", err))
		} else {
			a.appLogger.Info("Приложение успешно реконфигурировано")
		}
	}
}

func (a *App) reconfigure(cfg *config.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.appLogger.Info("Начало реконфигурации приложения")

	// Создаем новые компоненты
	lb, err := loadbalancer.New(cfg.LoadBalancer, a.appLogger)
	if err != nil {
		return fmt.Errorf("failed to create load balancer: %w", err)
	}
	a.appLogger.Info(fmt.Sprintf("Создан новый балансировщик нагрузки (метод: %s)", cfg.LoadBalancer.Method))

	rLim := ratelimit.NewTokenBucket(cfg.RateLimiter.TokenBucket.Rate, cfg.RateLimiter.TokenBucket.Burst)
	a.appLogger.Info(fmt.Sprintf("Создан новый rate limiter (rate: %.2f, burst: %d)",
		cfg.RateLimiter.TokenBucket.Rate,
		cfg.RateLimiter.TokenBucket.Burst))

	// Создаем новый прокси
	newProxy := transport.NewProxy(lb, rLim, a.appLogger)
	a.appLogger.Info("Создан новый прокси-сервер")

	// Если у нас уже есть прокси, gracefully останавливаем его
	if a.proxy != nil {
		a.appLogger.Info("Обнаружен работающий прокси, выполняем горячую замену")

		// Запускаем новый прокси
		if err := newProxy.Start(a.port); err != nil {
			return fmt.Errorf("failed to start new proxy: %w", err)
		}
		a.appLogger.Info("Новый прокси успешно запущен")

		// Останавливаем старый прокси
		if err := a.proxy.Stop(); err != nil {
			a.appLogger.Error(fmt.Sprintf("Ошибка при остановке старого прокси: %v", err))
		} else {
			a.appLogger.Info("Старый прокси успешно остановлен")
		}
	} else {
		// Первый запуск
		a.appLogger.Info("Выполняется первичный запуск прокси")
		if err := newProxy.Start(a.port); err != nil {
			return fmt.Errorf("failed to start proxy: %w", err)
		}
		a.appLogger.Info(fmt.Sprintf("Прокси успешно запущен на порту %s", a.port))
	}

	a.proxy = newProxy
	a.appLogger.Info("Реконфигурация приложения успешно завершена")
	return nil
}

func (a *App) Run() error {
	a.appLogger.Info(fmt.Sprintf("Приложение запущено и готово к работе на порту %s", a.port))

	// Создаем канал для сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Логируем полученный сигнал
	sig := <-sigChan
	a.appLogger.Info(fmt.Sprintf("Получен сигнал завершения работы: %v", sig))

	// Graceful shutdown
	a.mu.Lock()
	defer a.mu.Unlock()

	a.appLogger.Info("Начало graceful shutdown")

	if a.proxy != nil {
		if err := a.proxy.Stop(); err != nil {
			a.appLogger.Error(fmt.Sprintf("Ошибка при остановке прокси: %v", err))
		} else {
			a.appLogger.Info("Прокси успешно остановлен")
		}
	}

	if err := a.configManager.Close(); err != nil {
		a.appLogger.Error(fmt.Sprintf("Ошибка при закрытии менеджера конфигурации: %v", err))
	} else {
		a.appLogger.Info("Менеджер конфигурации успешно закрыт")
	}

	a.appLogger.Info("Приложение успешно завершило работу")
	return nil
}

func Run(configPath, port string) error {
	app, err := NewApp(configPath, port)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	return app.Run()
}
