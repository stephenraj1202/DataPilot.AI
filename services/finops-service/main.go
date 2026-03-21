package main

import (
	"log"
	"time"

	"github.com/finops-platform/finops-service/handlers"
	"github.com/finops-platform/finops-service/middleware"
	"github.com/finops-platform/shared/config"
	"github.com/finops-platform/shared/database"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load("../../config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	db, err := database.NewConnection(database.DBConfig{
		Host:         cfg.Database.Host,
		Port:         cfg.Database.Port,
		Username:     cfg.Database.Username,
		Password:     cfg.Database.Password,
		DatabaseName: cfg.Database.DatabaseName,
		MaxOpenConns: 25,
		MaxIdleConns: 5,
		MaxLifetime:  5 * time.Minute,
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	emailCfg := handlers.EmailConfig{
		SMTPHost:        cfg.Mail.DefaultSMTPHost,
		SMTPPort:        cfg.Mail.DefaultSMTPPort,
		SMTPUsername:    cfg.Mail.SMTPUsername,
		SMTPPassword:    cfg.Mail.SMTPPassword,
		FromEmail:       cfg.Mail.DefaultFromEmail,
		SuperAdminEmail: cfg.Mail.SuperAdminEmail,
	}

	aesKey := cfg.Encryption.AESKey

	// Start background schedulers
	syncScheduler := &handlers.SyncScheduler{DB: db.DB, AESKey: aesKey}
	syncScheduler.Start()

	anomalyDetector := &handlers.AnomalyDetector{DB: db.DB, EmailCfg: emailCfg}
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		time.Sleep(5 * time.Minute)
		anomalyDetector.DetectAnomalies()
		for range ticker.C {
			anomalyDetector.DetectAnomalies()
		}
	}()

	recScheduler := &handlers.RecommendationScheduler{DB: db.DB}
	recScheduler.Start()

	// Start report scheduler (checks every minute for due schedules)
	reportScheduler := &handlers.ReportScheduler{DB: db.DB, EmailCfg: emailCfg, AESKey: aesKey, Scheduler: syncScheduler}
	reportScheduler.Start()

	// Initialize handlers
	cloudAccountHandler := &handlers.CloudAccountHandler{
		DB:        db.DB,
		AESKey:    aesKey,
		EmailCfg:  emailCfg,
		Scheduler: syncScheduler,
	}
	costHandler := &handlers.CostHandler{DB: db.DB}
	anomalyHandler := &handlers.AnomalyHandler{DB: db.DB}
	recommendationHandler := &handlers.RecommendationHandler{DB: db.DB}
	reportHandler := &handlers.ReportHandler{DB: db.DB, EmailCfg: emailCfg, AESKey: aesKey, Scheduler: syncScheduler}

	// Initialize Gin router
	router := gin.Default()

	// Inject user/account context from gateway headers on all routes
	router.Use(middleware.InjectContext())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "finops-service",
		})
	})

	// FinOps endpoints — all require an authenticated account
	finops := router.Group("/finops")
	finops.Use(middleware.RequireAccount())
	{
		// Cloud accounts
		finops.POST("/cloud-accounts", cloudAccountHandler.ConnectCloudAccount)
		finops.GET("/cloud-accounts", cloudAccountHandler.ListCloudAccounts)
		finops.GET("/cloud-accounts/:id/resources", cloudAccountHandler.GetCloudAccountResources)
		finops.GET("/cloud-accounts/:id/vm-resources", cloudAccountHandler.GetCloudAccountVMResources)
		finops.GET("/cloud-accounts/:id/tiles", cloudAccountHandler.GetResourceTiles)
		finops.PUT("/cloud-accounts/:id", cloudAccountHandler.UpdateCloudAccount)
		finops.DELETE("/cloud-accounts/:id", cloudAccountHandler.DeleteCloudAccount)
		finops.POST("/cloud-accounts/:id/sync", cloudAccountHandler.SyncCloudAccount)

		// Cost summary
		finops.GET("/costs/summary", costHandler.GetCostSummary)

		// Anomalies
		finops.GET("/anomalies", anomalyHandler.ListAnomalies)
		finops.POST("/anomalies/:id/acknowledge", anomalyHandler.AcknowledgeAnomaly)

		// Recommendations
		finops.GET("/recommendations", recommendationHandler.GetRecommendations)

		// Report schedules
		finops.POST("/reports/send", reportHandler.SendReportNow)
		finops.POST("/reports/schedules", reportHandler.CreateSchedule)
		finops.GET("/reports/schedules", reportHandler.ListSchedules)
		finops.PUT("/reports/schedules/:id", reportHandler.UpdateSchedule)
		finops.DELETE("/reports/schedules/:id", reportHandler.DeleteSchedule)
	}

	// Start server
	log.Println("FinOps Service starting on port 8083")
	if err := router.Run(":8083"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
