package testutil

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	*postgres.PostgresContainer
	DB *sqlx.DB
}

func SetupContainerAndMigrations(ctx context.Context) (*PostgresContainer, error) {
	pgContainer, err := postgres.Run(
		ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("mixologist_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		return nil, err
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}

	testDB, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Println("ошибка при соединении с базой данных", err)
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}

	if err = applyMigrations(testDB); err != nil {
		_ = testDB.Close()
		testDB = nil
		_ = pgContainer.Terminate(ctx)
		pgContainer = nil
		return nil, err
	}
	return &PostgresContainer{
		PostgresContainer: pgContainer,
		DB:                testDB,
	}, nil
}

func applyMigrations(db *sqlx.DB) error {
	migrationsDir := filepath.Join("..", "..", "migrations")
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, migrationFile := range files {
		sqlBytes, readErr := os.ReadFile(migrationFile)
		if readErr != nil {
			return readErr
		}
		if _, execErr := db.Exec(string(sqlBytes)); execErr != nil {
			return execErr
		}
	}
	return nil
}

func TruncateIngredients(db *sqlx.DB) error {
	_, err := db.Exec(`TRUNCATE TABLE ingredients RESTART IDENTITY CASCADE`)
	return err
}
