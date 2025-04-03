package controllers

import (
	"fmt"
	"log"
	"os"
	"zeroshare-backend/structs"

	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDatabase() *gorm.DB {
	//Create a new Postgresql database connection
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	sslMode := os.Getenv("DB_SSLMODE")
	timeZone := os.Getenv("DB_TIMEZONE")

	// Build the DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, sslMode, timeZone)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		log.Fatal("Failed instrument GORM: ", err)
	}

	// Enable the extension for generating UUIDs in Postgres if not already enabled
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")

	db.Migrator().DropTable(&structs.Peer{})

	// AutoMigrate the User schema
	db.AutoMigrate(&structs.User{})
	db.AutoMigrate(&structs.Device{})

	return db
}
