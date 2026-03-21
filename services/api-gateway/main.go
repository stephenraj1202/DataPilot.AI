package main

import (
	"log"
	"time"

	"github.com/finops-platform/api-gateway/circuitbreaker"
	"github.com/finops-platform/api-gateway/middleware"
	"github.com/finops-platform/api-gateway/proxy"
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

	// Circuit breaker registry: 10 failures → open, 10s timeout (relaxed for local dev)
	cbRegistry := circuitbreaker.NewRegistry(10, 10*time.Second)

	// Middleware
	authMW := &middleware.AuthMiddleware{
		JWTSecret: cfg.Auth.JWTSecret,
		DB:        db.DB,
	}
	rateLimitMW := &middleware.RateLimitMiddleware{
		DB: db.DB,
	}
	requestLogger := &middleware.RequestLogger{
		DB: db.DB,
	}

	// Reverse proxy
	rp := proxy.New(cbRegistry)

	// Upstream service base URLs — read from config.ini [services]
	authBase := cfg.Services.AuthServiceURL
	billingBase := cfg.Services.BillingServiceURL
	finopsBase := cfg.Services.FinOpsServiceURL
	aiBase := cfg.Services.AIQueryEngineURL

	// Router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.Services.FrontendURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	router.Use(requestLogger.Log())

	// Health check — no auth required
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":           "healthy",
			"service":          "api-gateway",
			"circuit_breakers": cbRegistry.Status(),
		})
	})

	// Circuit breaker reset — for dev/ops use
	router.POST("/admin/circuit-breakers/:service/reset", func(c *gin.Context) {
		svc := c.Param("service")
		cbRegistry.Get(svc).Reset()
		c.JSON(200, gin.H{"reset": svc, "circuit_breakers": cbRegistry.Status()})
	})

	// Stripe webhook — separate prefix to avoid wildcard conflict with /api/billing/*path
	router.POST("/webhook/billing", rp.ForwardToPath("billing-service", billingBase, "/billing/webhook"))

	// Public auth routes — no authentication required
	router.Any("/auth/*path", rp.Forward("auth-service", authBase))

	// Protected API routes
	api := router.Group("/api")
	api.Use(authMW.Authenticate())
	api.Use(rateLimitMW.Limit())
	{
		// Auth Service routes
		api.Any("/auth/*path", rp.ForwardStripPrefix("auth-service", authBase, "/api"))

		// Billing Service routes
		api.Any("/billing/*path", rp.ForwardStripPrefix("billing-service", billingBase, "/api"))

		// FinOps Service routes
		api.Any("/finops/*path", rp.ForwardStripPrefix("finops-service", finopsBase, "/api"))

		// AI Query Engine routes
		api.Any("/query/*path", rp.ForwardStripPrefix("ai-query-engine", aiBase, "/api"))
	}

	log.Println("API Gateway starting on port 8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
