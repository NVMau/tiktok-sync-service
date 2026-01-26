package database

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/user/sync-tiktok-mps/internal/models"
)

var DB *gorm.DB

func Connect(databaseURL string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	log.Println("Database connected successfully")
	return nil
}

func Migrate() error {
	log.Println("Running database migrations...")

	// Create schema if not exists
	err := DB.Exec("CREATE SCHEMA IF NOT EXISTS tiktok_sync").Error
	if err != nil {
		return err
	}

	// Migrate all tables (GORM will auto-add new columns/tables)
	// Order matters: parent tables first, then child tables
	if err = DB.AutoMigrate(
		// Shop management
		&models.Shop{},
		&models.OAuthToken{},

		// Products & SKUs (NEW)
		&models.Product{},
		&models.SKU{},

		// Orders
		&models.Order{},
		&models.OrderItem{},

		// Job queue
		&models.WebhookEvent{},
		&models.SyncJob{},
	); err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}
