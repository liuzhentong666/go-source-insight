package database

import (
	"fmt"
	"net"
	"net/url"
	"os"

	"go-ai-study/api/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {
	// 从配置中获取数据库连接信息
	cfg := GetDBConfig()
	return InitDBWithConfig(cfg)
}

func InitDBWithConfig(cfg *DBConfig) error {
	db, err := Open(cfg)
	if err != nil {
		return err
	}
	if err = Migrate(db); err != nil {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		return err
	}
	DB = db
	return nil
}
func Open(cfg *DBConfig) (*gorm.DB, error) {
	dsn := connectionDSN(cfg)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("database connection unavailable")
	}

	return db, nil
}

// URL encoding preserves injected credentials containing spaces or quotes.
// Interpolating those values into a keyword DSN changes their meaning.
func connectionDSN(cfg *DBConfig) string {
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.User, cfg.Password), Host: net.JoinHostPort(cfg.Host, cfg.Port), Path: "/" + cfg.DBName}
	query := url.Values{"sslmode": {"disable"}, "connect_timeout": {"10"}}
	u.RawQuery = query.Encode()
	return u.String()
}
func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&models.User{}, &models.Project{}, &models.Analysis{}, &models.Job{})
	if err != nil {
		return fmt.Errorf("database migration failed")
	}

	return nil
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
