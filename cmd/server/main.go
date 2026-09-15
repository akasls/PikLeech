package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/api"
	"pikpak-manager/internal/apikey"
	"pikpak-manager/internal/audit"
	"pikpak-manager/internal/auth"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/dashboard"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/fileagg"
	"pikpak-manager/internal/offline"
	"pikpak-manager/internal/scheduler"
	"pikpak-manager/internal/settings"
	"pikpak-manager/web"
)

func main() {
	log.Println("==================================================")
	log.Println("     PikPak Multi-Account Aggregator Web System    ")
	log.Println("==================================================")

	cfg := config.Load()
	log.Printf("[CONFIG] Data directory: %s", cfg.DataDir)
	log.Printf("[CONFIG] Database file: %s", cfg.DBPath)

	database, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Initialize services
	settingsSvc := settings.NewService(database)
	accSvc := account.NewService(database, cfg.AppSecret)
	accountScheduler := scheduler.NewAccountScheduler(accSvc, settingsSvc)
	offSvc := offline.NewService(database, accSvc, accountScheduler)
	fileSvc := fileagg.NewService(database, accSvc)
	authSvc := auth.NewService(database, cfg.AppSecret)
	apiKeySvc := apikey.NewService(database)
	auditSvc := audit.NewService(database)
	dashSvc := dashboard.NewService(database)

	// Start background task poller
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	offSvc.StartPoller(ctx, 6*time.Second)

	// Daily quota check ticker (every 10 minutes)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = accSvc.AutoResetDailyQuotas()
			}
		}
	}()

	// Periodic Auto Cleanup of expired offline downloads (every 30 minutes)
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if set, err := settingsSvc.GetSettings(ctx); err == nil && set.AutoCleanupEnabled && set.AutoCleanupDays > 0 {
					_, _ = offSvc.CleanExpiredTasks(ctx, set.AutoCleanupDays)
				}
			}
		}
	}()

	server := api.NewServer(
		accSvc,
		fileSvc,
		offSvc,
		authSvc,
		apiKeySvc,
		auditSvc,
		dashSvc,
		&web.DistFS,
		settingsSvc,
	)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.Engine,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally 0 to allow uninterrupted video streaming and large file transfers
	}

	go func() {
		log.Printf("[SERVER] PikPak Manager listening on http://0.0.0.0:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SERVER] Shutting down PikPak Manager gracefully...")
	offSvc.StopPoller()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[SERVER] System exited cleanly.")
}
