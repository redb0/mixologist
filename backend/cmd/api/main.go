package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redb0/mixologist/internal/handlers"
	"github.com/redb0/mixologist/internal/repository"
	"github.com/redb0/mixologist/internal/services"
)

func initDB() (*sqlx.DB, error) {
	uri := os.Getenv("DB_URL")
	db, err := sqlx.Open("postgres", uri)
	if err != nil {
		log.Println("Ошибка при соединении с базой данных", err)
		return nil, err
	}

	db.SetMaxOpenConns(25)                 // Максимальное количество открытых соединений
	db.SetMaxIdleConns(5)                  // Максимальное количество неиспользуемых соединений
	db.SetConnMaxLifetime(5 * time.Minute) // Максимальное время использования соединения
	db.SetConnMaxIdleTime(2 * time.Minute) // Максимальное время ожидания соединения

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Println("Не удалось подключиться к базе данных", err)
		_ = db.Close()
		return nil, err
	}

	log.Println("Успешное подключение к базе данных")
	return db, nil
}

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer func() { _ = db.Close() }()

	ingredientRepository := repository.NewIngredientRepository(db)
	ingredientService := services.NewIngredientService(ingredientRepository)
	ingredientController := handlers.NewIngredientController(ingredientService)

	router := gin.Default()
	// router.GET("/ingredients", GetIngredients)
	// router.GET("/ingredients/:id", GetIngredient)
	router.POST("/ingredients", ingredientController.CreateIngredient)
	// router.PATCH("/ingredients/:id", UpdateIngredient)
	// router.DELETE("/ingredients/:id", DeleteIngredient)
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Ошибка запуска HTTP-сервера: %v", err)
	}
}
