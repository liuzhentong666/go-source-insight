package database

import (
	"fmt"
	"log"
	"os"

	"github.com/go-ai-study/api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	// 从配置中获取数据库连接信息
	cfg := GetDBConfig()
	InitDBWithConfig(cfg)
}

func InitDBWithConfig(cfg *DBConfig) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 自动迁移数据库表
	err = DB.AutoMigrate(&models.User{}, &models.Project{}, &models.Analysis{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database connection established and migrations completed")
}

func GetDBConfig() *DBConfig {
	// 从环境变量读取配置，如果不存在则使用默认值
	return &DBConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "password"),
		DBName:   getEnv("DB_NAME", "goaiinsight"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}
