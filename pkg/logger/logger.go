package logger

import (
	"fmt"
	"github.com/fatih/color"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

// CustomZapLogger - структура для логгера
type CustomZapLogger struct {
	logger *zap.Logger
}

// LoggerConfig - конфигурация для логгера
type LoggerConfig struct {
	LogLevel    string
	NodeIP      string
	PodIP       string
	ServiceName string
}

// NewCustomZapLogger - конструктор для создания нового логгера
func NewCustomZapLogger(cfg *LoggerConfig) *CustomZapLogger {
	// Настройка минимального уровня логирования
	var level zapcore.Level
	switch cfg.LogLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	// Проверка существования директории "logs"
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		if err := os.MkdirAll("logs", 0755); err != nil {
			panic(fmt.Sprintf("unable to create logs directory: %v", err))
		}
	}

	// Конфигурация для записи в файл
	fileEncoderConfig := zap.NewProductionEncoderConfig()
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Читаемый формат времени
	fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)

	file, err := os.OpenFile("logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("unable to open log file: %v", err))
	}
	fileWriter := zapcore.AddSync(file)

	// Конфигурация для консоли
	consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
	consoleEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // Добавляем источник лога (файл и строка)
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
	consoleWriter := zapcore.AddSync(os.Stdout)

	// Создаем Core для файла (с глобальными полями)
	fileCore := zapcore.NewCore(fileEncoder, fileWriter, level).With(
		[]zapcore.Field{
			zap.String("NodeIP", cfg.NodeIP),
			zap.String("PodIP", cfg.PodIP),
			zap.String("ServiceName", cfg.ServiceName),
		},
	)

	// Создаем Core для консоли (без глобальных полей)
	consoleCore := zapcore.NewCore(consoleEncoder, consoleWriter, level)

	// Комбинированный логгер
	logger := zap.New(zapcore.NewTee(fileCore, consoleCore), zap.AddCaller(), zap.AddCallerSkip(1))

	return &CustomZapLogger{logger: logger}
}

// Debug - обертка для лога уровня Debug
func (l *CustomZapLogger) Debug(msg string, fields ...zap.Field) {
	color.Set(color.FgCyan)
	defer color.Unset()
	fmt.Println("[DEBUG] " + msg)
	l.logger.Debug(msg, fields...)
}

// Info - обертка для лога уровня Info
func (l *CustomZapLogger) Info(msg string, fields ...zap.Field) {
	color.Set(color.FgGreen)
	defer color.Unset()
	fmt.Println("[INFO] " + msg)
	l.logger.Info(msg, fields...)
}

// Warn - обертка для лога уровня Warn
func (l *CustomZapLogger) Warn(msg string, fields ...zap.Field) {
	color.Set(color.FgYellow)
	defer color.Unset()
	fmt.Println("[WARN] " + msg)
	l.logger.Warn(msg, fields...)
}

// Error - обертка для лога уровня Error
func (l *CustomZapLogger) Error(msg string, fields ...zap.Field) {
	color.Set(color.FgRed)
	defer color.Unset()
	fmt.Println("[ERROR] " + msg)
	l.logger.Error(msg, fields...)
}

// Fatal - обертка для лога уровня Fatal
func (l *CustomZapLogger) Fatal(msg string, fields ...zap.Field) {
	color.Set(color.FgHiRed)
	defer color.Unset()
	fmt.Println("[FATAL] " + msg)
	l.logger.Fatal(msg, fields...)
}

// Printf - форматированный вывод в консоль и лог
func (l *CustomZapLogger) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	color.Set(color.FgMagenta)
	defer color.Unset()
	fmt.Println("[INFO] " + msg)
	l.logger.Info(msg)
}