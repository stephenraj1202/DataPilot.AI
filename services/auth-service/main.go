package main

import (
	"log"
	"time"

	"github.com/finops-platform/auth-service/handlers"
	"github.com/finops-platform/auth-service/middleware"
	"github.com/finops-platform/auth-service/utils"
	"github.com/finops-platform/shared/config"
	"github.com/finops-platform/shared/database"
	"github.com/gin-contrib/cors"
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

	// Initialize enhanced email service (custom SMTP per account + retry logic)
	emailService := &utils.EnhancedEmailService{
		DB: db.DB,
		DefaultSMTP: utils.EmailConfig{
			SMTPHost:     cfg.Mail.DefaultSMTPHost,
			SMTPPort:     cfg.Mail.DefaultSMTPPort,
			SMTPUsername: cfg.Mail.SMTPUsername,
			SMTPPassword: cfg.Mail.SMTPPassword,
			FromEmail:    cfg.Mail.DefaultFromEmail,
			SuperAdminCC: cfg.Mail.SuperAdminEmail,
		},
		AESKey: cfg.Encryption.AESKey,
	}

	// Initialize JWT service
	jwtService := &utils.JWTService{
		SecretKey: cfg.Auth.JWTSecret,
	}

	// Initialize handlers
	registerHandler := &handlers.RegisterHandler{
		DB:          db.DB,
		EmailSender: emailService,
	}

	verifyEmailHandler := &handlers.VerifyEmailHandler{
		DB: db.DB,
	}

	loginHandler := &handlers.LoginHandler{
		DB:         db.DB,
		JWTService: jwtService,
	}

	refreshHandler := &handlers.RefreshHandler{
		DB:         db.DB,
		JWTService: jwtService,
	}

	passwordResetHandler := &handlers.PasswordResetHandler{
		DB:          db.DB,
		EmailSender: emailService,
	}

	apiKeyHandler := &handlers.APIKeyHandler{
		DB: db.DB,
	}

	smtpHandler := &handlers.SMTPHandler{
		DB:     db.DB,
		AESKey: cfg.Encryption.AESKey,
	}

	auditHandler := &handlers.AuditHandler{
		DB: db.DB,
	}

	oauthHandler := &handlers.OAuthHandler{
		DB:         db.DB,
		JWTService: jwtService,
	}

	otpHandler := &handlers.OTPHandler{
		DB:          db.DB,
		EmailSender: emailService,
		JWTService:  jwtService,
	}

	// Initialize auth middleware
	authMiddleware := &middleware.AuthMiddleware{
		JWTService: jwtService,
	}

	// Initialize Gin router
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "auth-service",
		})
	})

	// Auth endpoints
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", registerHandler.Register)
		authGroup.GET("/verify-email", verifyEmailHandler.VerifyEmail)
		authGroup.POST("/login", loginHandler.Login)
		authGroup.POST("/refresh", refreshHandler.Refresh)
		authGroup.POST("/forgot-password", passwordResetHandler.ForgotPassword)
		authGroup.POST("/reset-password", passwordResetHandler.ResetPassword)
		authGroup.POST("/google", oauthHandler.GoogleAuth)
		authGroup.POST("/send-otp", otpHandler.SendOTP)
		authGroup.POST("/verify-otp", otpHandler.VerifyOTP)

		// API key management (requires authentication)
		apiKeys := authGroup.Group("/api-keys")
		apiKeys.Use(authMiddleware.ValidateToken())
		{
			apiKeys.POST("", apiKeyHandler.CreateAPIKey)
			apiKeys.GET("", apiKeyHandler.ListAPIKeys)
			apiKeys.DELETE("/:id", apiKeyHandler.RevokeAPIKey)
		}
	}

	// Settings endpoints (require authentication)
	settingsGroup := router.Group("/settings")
	settingsGroup.Use(authMiddleware.ValidateToken())
	{
		settingsGroup.POST("/smtp", smtpHandler.SaveSMTPConfig)
		settingsGroup.GET("/smtp", smtpHandler.GetSMTPConfig)
	}

	// Admin endpoints (Super_Admin only)
	adminGroup := router.Group("/admin")
	adminGroup.Use(authMiddleware.ValidateToken())
	adminGroup.Use(middleware.RequirePermission(db.DB, "admin:audit_logs"))
	{
		adminGroup.GET("/audit-logs", auditHandler.ListAuditLogs)
	}

	// Start server
	log.Println("Auth Service starting on port 8081")
	if err := router.Run(":8081"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
