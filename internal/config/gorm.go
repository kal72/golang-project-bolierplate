package config

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	applog "golang-project-boilerplate/internal/shared/logger"
)

func NewDatabase(config *Config, appLog *applog.Logger) *gorm.DB {
	username := config.Database.Username
	password := config.Database.Password
	host := config.Database.Host
	port := config.Database.Port
	database := config.Database.Name
	idleConnection := config.Database.Pool.Idle
	maxConnection := config.Database.Pool.Max
	maxLifeTimeConnection := config.Database.Pool.Lifetime

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection, err := db.DB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection.SetMaxIdleConns(idleConnection)
	connection.SetMaxOpenConns(maxConnection)
	connection.SetConnMaxLifetime(time.Second * time.Duration(maxLifeTimeConnection))

	return db
}
