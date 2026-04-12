package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	Database DatabaseConfig
	JWT      JWTConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type JWTConfig struct {
	SecretKey string
}

func Load() (*Config, error) {
	// 加载.env文件
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found")
	}

	return &Config{
		Port: os.Getenv("PORT"),
		Database: DatabaseConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     os.Getenv("DB_PORT"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   os.Getenv("DB_NAME"),
		},
		JWT: JWTConfig{
			SecretKey: os.Getenv("JWT_SECRET"),
		},
	}, nil
}

func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("PORT environment variable is required")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST environment variable is required")
	}
	if c.Database.Port == "" {
		return fmt.Errorf("DB_PORT environment variable is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("DB_USER environment variable is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD environment variable is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("DB_NAME environment variable is required")
	}
	if c.JWT.SecretKey == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}
	return nil
}