package database

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"time"
)

var Database *gorm.DB

func init() {
	dsn := "root:123456@tcp(127.0.0.1:3306)/employees?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	Database, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("error while open database connection", err)
		os.Exit(-1)
	}

	db, err := Database.DB()
	if err != nil {
		fmt.Println("error while fetching database", err)
		os.Exit(-1)
	}

	db.SetMaxOpenConns(200)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(1 * time.Minute)
	db.SetConnMaxLifetime(5 * time.Minute)
}
