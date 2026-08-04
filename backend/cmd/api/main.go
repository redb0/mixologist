package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redb0/mixologist/internal/config"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/repository"
	"github.com/redb0/mixologist/internal/router"
	"github.com/redb0/mixologist/internal/services"
)

func initLogger(level string) {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})))
}

func initDB(uri string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", uri)
	if err != nil {
		slog.Error("ошибка при соединении с базой данных", "err", err)
		return nil, err
	}

	db.SetMaxOpenConns(25)                 // Максимальное количество открытых соединений
	db.SetMaxIdleConns(5)                  // Максимальное количество неиспользуемых соединений
	db.SetConnMaxLifetime(5 * time.Minute) // Максимальное время использования соединения
	db.SetConnMaxIdleTime(2 * time.Minute) // Максимальное время ожидания соединения

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("не удалось подключиться к базе данных", "err", err)
		_ = db.Close()
		return nil, err
	}

	slog.Info("успешное подключение к базе данных")
	return db, nil
}

func main() {
	initLogger("info")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("ошибка конфигурации", "err", err)
		os.Exit(1)
	}

	initLogger(cfg.LogLevel)
	gin.SetMode(cfg.GinMode)

	db, err := initDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("ошибка инициализации базы данных", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	ingredientRepository := repository.NewIngredientRepository(db)
	ingredientService := services.NewIngredientService(ingredientRepository)
	ingredientController := handlers.NewIngredientController(ingredientService)
	healthController := handlers.NewHealthController(db)

	app := router.New(router.Dependencies{
		HealthController:     healthController,
		IngredientController: ingredientController,
	})

	if err := app.Run(cfg.HTTPAddr); err != nil {
		slog.Error("ошибка запуска HTTP-сервера", "err", err)
		os.Exit(1)
	}
}
