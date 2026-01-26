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

	// Disable foreign key constraints during migration
	DB.Exec("SET session_replication_role = replica")
	defer DB.Exec("SET session_replication_role = DEFAULT")

	// Migrate all tables one by one to avoid issues
	tables := []interface{}{
		&models.Shop{},
		&models.OAuthToken{},
		&models.Product{},
		&models.SKU{},
		&models.Order{},
		&models.OrderItem{},
		&models.WebhookEvent{},
		&models.SyncJob{},
	}

	for _, table := range tables {
		if err = DB.AutoMigrate(table); err != nil {
			log.Printf("Warning: migration error for %T: %v", table, err)
			// Continue with other tables
		}
	}

	log.Println("Database migrations completed")
	return nil
}
