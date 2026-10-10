package main

import (
	"context"
	"errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go-ai-study/api/config"
	"go-ai-study/api/database"
	"go-ai-study/api/handlers"
	"go-ai-study/api/routes"
	"go-ai-study/api/service"
	"go-ai-study/api/storage"
	"go-ai-study/api/utils"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("API stopped: %v", err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err = cfg.Validate(); err != nil {
		return err
	}
	signer, err := utils.NewTokens(cfg.JWT.SecretKey)
	if err != nil {
		return err
	}
	db, err := database.Open(&database.DBConfig{Host: cfg.Database.Host, Port: cfg.Database.Port, User: cfg.Database.User, Password: cfg.Database.Password, DBName: cfg.Database.DBName})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("database connection unavailable")
	}
	clean := true
	defer func() {
		if clean {
			sqlDB.Close()
		}
	}()
	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	repo := storage.NewPostgres(db)
	instance, err := storage.StartInstance(startupCtx, sqlDB, func(ctx context.Context) error {
		if err := database.Migrate(db.WithContext(ctx)); err != nil {
			return err
		}
		if err := repo.RecoverInterrupted(ctx); err != nil {
			return errors.New("interrupted job recovery failed")
		}
		return nil
	})
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		if clean {
			instance.Close()
		}
	}()
	cancel()
	database.DB = db // Existing metadata/history handlers use the same owned database.
	repo = storage.NewPostgresWithInstance(db, instance)
	serviceCtx, stopService := context.WithCancel(context.Background())
	defer stopService()
	svc, err := service.NewSource(serviceCtx, repo, cfg.SourceRoot, cfg.JobWorkers, cfg.JobQueueSize, service.RAGSettings{JobTimeout: cfg.JobTimeout, SourceBindings: cfg.SourceBindings, LocalRoot: cfg.LocalRoot})
	if err != nil {
		return errors.New("source service initialization failed")
	}
	router := gin.New()
	router.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		log.Print("HTTP handler failed")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "operation_failed", "error": "operation failed"})
	}))
	if len(cfg.CORSOrigins) > 0 {
		router.Use(cors.New(cors.Config{AllowOrigins: cfg.CORSOrigins, AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"}, AllowHeaders: []string{"Content-Type", "Authorization"}, MaxAge: time.Hour}))
	}
	routes.RegisterRoutes(router, &handlers.SourceHandler{Service: svc, Tokens: signer})
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	failures := make(chan error, 2)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			failures <- errors.New("HTTP listener unavailable")
		}
	}()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-serviceCtx.Done():
				return
			case <-ticker.C:
				checkCtx, cancel := context.WithTimeout(serviceCtx, 3*time.Second)
				err := instance.Check(checkCtx)
				cancel()
				if err != nil {
					failures <- errors.New("instance ownership connection lost")
					return
				}
			}
		}
	}()
	log.Print("API ready")
	var cause error
	select {
	case <-ctx.Done():
	case cause = <-failures:
	}
	// Stop admissions/model calls immediately, then drain HTTP and workers within
	// one deadline. If dependencies ignore cancellation, exit without unlocking
	// early; process termination releases the PostgreSQL session and handles.
	stopService()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- svc.Shutdown(shutdownCtx) }()
	httpErr := server.Shutdown(shutdownCtx)
	if httpErr != nil {
		_ = server.Close()
	}
	select {
	case err = <-stopped:
	case <-shutdownCtx.Done():
		err = shutdownCtx.Err()
	}
	if err != nil || httpErr != nil {
		clean = false
		return errors.New("shutdown deadline exceeded; unfinished jobs require startup reconciliation")
	}
	return cause
}
