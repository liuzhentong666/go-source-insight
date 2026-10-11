package config

import (
	"fmt"
	"go-ai-study/api/sourcefiles"
	"go-ai-study/api/utils"
	"go-ai-study/internal/localfiles"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	SourceAccessMode string
	SourceReader     localfiles.Provider
	Port             string
	Database         DatabaseConfig
	JWT              JWTConfig
	SourceRoot       string
	SourceBindings   []sourcefiles.Binding
	LocalRoot        *sourcefiles.LocalRoot
	JobWorkers       int
	JobQueueSize     int
	JobTimeout       time.Duration
	ShutdownTimeout  time.Duration
	CORSOrigins      []string
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

	workers, err := boundedInt("JOB_WORKERS", 2, 32)
	if err != nil {
		return nil, err
	}
	queueSize, err := boundedInt("JOB_QUEUE_SIZE", 16, 1024)
	if err != nil {
		return nil, err
	}
	root := os.Getenv("SOURCE_ROOT")
	mode := os.Getenv("SOURCE_ACCESS_MODE")
	if mode == "" {
		mode = "host"
	}
	if mode != "host" && mode != "mapped" {
		return nil, fmt.Errorf("SOURCE_ACCESS_MODE must be host or mapped")
	}
	reader, err := localfiles.FromEnvironment()
	if err != nil {
		return nil, err
	}
	if root == "" {
		root = "sources"
	}
	timeout, err := boundedInt("JOB_TIMEOUT_SECONDS", 900, 86400)
	if err != nil {
		return nil, err
	}
	shutdown, err := boundedInt("SHUTDOWN_TIMEOUT_SECONDS", 30, 300)
	if err != nil {
		return nil, err
	}
	origins := []string{}
	sources, err := sourcefiles.LoadSourceConfig(os.Getenv("SOURCE_BINDINGS_FILE"))
	if err != nil {
		return nil, err
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if strings.TrimSpace(origin) != "" {
			origins = append(origins, strings.TrimSpace(origin))
		}
	}
	return &Config{
		SourceAccessMode: mode, SourceReader: reader,
		JobTimeout: time.Duration(timeout) * time.Second, ShutdownTimeout: time.Duration(shutdown) * time.Second, CORSOrigins: origins,
		SourceRoot: root, JobWorkers: workers, JobQueueSize: queueSize,
		SourceBindings: sources.Bindings, LocalRoot: sources.LocalRoot,
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
	if strings.TrimSpace(c.SourceRoot) == "" {
		return fmt.Errorf("SOURCE_ROOT must not be empty")
	}
	if c.JobWorkers < 1 || c.JobWorkers > 32 {
		return fmt.Errorf("JOB_WORKERS must be between 1 and 32")
	}
	if c.JobQueueSize < 1 || c.JobQueueSize > 1024 {
		return fmt.Errorf("JOB_QUEUE_SIZE must be between 1 and 1024")
	}
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
	if _, err := utils.NewTokens(c.JWT.SecretKey); err != nil {
		return err
	}
	if c.JobTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return fmt.Errorf("invalid execution timeout")
	}
	for _, origin := range c.CORSOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || strings.ContainsAny(origin, "*\r\n") || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("CORS_ORIGINS must contain explicit HTTP origins")
		}
	}
	return nil
}

func boundedInt(name string, fallback, max int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > max {
		return 0, fmt.Errorf("%s must be between 1 and %d", name, max)
	}
	return n, nil
}
