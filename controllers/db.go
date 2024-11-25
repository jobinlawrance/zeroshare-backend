package controllers

import (
	"log"
	"zeroshare-backend/structs"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDatabase() *gorm.DB {
	//Create a new Postgresql database connection
	dsn := "host=localhost user=postgres password=root dbname=zeroshare port=5432 sslmode=disable TimeZone=Asia/Kolkata"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	// Enable the extension for generating UUIDs in Postgres if not already enabled
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")

	// AutoMigrate the User schema
	db.AutoMigrate(&structs.User{})
	db.AutoMigrate(&structs.Peer{})
	db.AutoMigrate(&structs.Device{})
	return db
}