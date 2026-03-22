package main

import (
	"log"
	"time"

	"github.com/finops-platform/billing-service/handlers"
	"github.com/finops-platform/shared/config"
	"github.com/finops-platform/shared/database"
	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v76"
)

func main() {
	// Load configuration
	cfg, err := config.Load("../../config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Configure Stripe (always init, even if Razorpay is active — some local logic uses it)
	stripe.Key = cfg.Stripe.APIKey

	log.Printf("Payment mode: %s", cfg.Payment.Mode)

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

	// Seed subscription plans (pass price IDs from config so they get applied)
	if err := handlers.SeedPlans(db.DB, map[string]string{
		"base":       cfg.StripePlans.BasePriceID,
		"pro":        cfg.StripePlans.ProPriceID,
		"enterprise": cfg.StripePlans.EnterprisePriceID,
	}); err != nil {
		log.Printf("Warning: failed to seed subscription plans: %v", err)
	}

	emailCfg := handlers.EmailConfig{
		SMTPHost:        cfg.Mail.DefaultSMTPHost,
		SMTPPort:        cfg.Mail.DefaultSMTPPort,
		FromEmail:       cfg.Mail.DefaultFromEmail,
		SuperAdminEmail: cfg.Mail.SuperAdminEmail,
	}

	// Initialize handlers
	customerHandler := &handlers.CustomerHandler{DB: db.DB}
	subscriptionHandler := &handlers.SubscriptionHandler{
		DB:             db.DB,
		RazorpayKeyID:  cfg.Razorpay.KeyID,
		RazorpaySecret: cfg.Razorpay.KeySecret,
	}
	webhookHandler := &handlers.WebhookHandler{
		DB:            db.DB,
		WebhookSecret: cfg.Stripe.WebhookSecret,
		EmailCfg:      emailCfg,
	}
	invoiceHandler := &handlers.InvoiceHandler{
		DB:       db.DB,
		EmailCfg: emailCfg,
	}
	planLimitsHandler := &handlers.PlanLimitsHandler{DB: db.DB}
	checkoutHandler := &handlers.CheckoutHandler{
		DB:         db.DB,
		SuccessURL: cfg.Stripe.SuccessURL,
		CancelURL:  cfg.Stripe.CancelURL,
		Cfg:        cfg,
	}
	ubbHandler := &handlers.UBBHandler{
		DB:                db.DB,
		PaymentMode:       cfg.Payment.Mode,
		RazorpayKeyID:     cfg.Razorpay.KeyID,
		RazorpayKeySecret: cfg.Razorpay.KeySecret,
	}

	// Ensure UBB tables exist
	if err := handlers.EnsureUBBTable(db.DB); err != nil {
		log.Printf("Warning: failed to create UBB tables: %v", err)
	}

	// Backfill sub_item_price_cents for existing streams (runs once, fast) — Stripe only
	if cfg.Payment.Mode != "razorpay" {
		go handlers.SyncSubItemPrices(db.DB)
	}

	// Daily goroutine: snapshot active stream usage into ubb_billed_revenue
	go func() {
		// Run once immediately at startup, then every 24 hours
		handlers.SnapshotBilledRevenue(db.DB)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			handlers.SnapshotBilledRevenue(db.DB)
		}
	}()

	// Initialize Gin router
	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "billing-service",
		})
	})

	// Billing endpoints
	billing := router.Group("/billing")
	{
		billing.GET("/plans", handlers.GetPlans(db.DB))
		billing.GET("/payment-mode", func(c *gin.Context) {
			c.JSON(200, gin.H{"mode": cfg.Payment.Mode})
		})
		billing.GET("/subscription", subscriptionHandler.GetSubscription)
		billing.POST("/customers", customerHandler.CreateCustomer)
		billing.POST("/subscribe", subscriptionHandler.Subscribe)
		billing.PUT("/subscription", subscriptionHandler.UpdateSubscription)
		billing.POST("/webhook", webhookHandler.HandleWebhook)
		billing.GET("/invoices", invoiceHandler.ListInvoices)
		billing.GET("/invoices/:id/pdf", invoiceHandler.DownloadInvoicePDF)
		billing.GET("/plan-limits", planLimitsHandler.GetPlanLimits)
		billing.POST("/checkout", checkoutHandler.CreateCheckoutSession)
		billing.POST("/checkout/confirm", checkoutHandler.ConfirmCheckoutSession)

		// Razorpay (used when payment.mode=razorpay)
		razorpayHandler := &handlers.RazorpayCheckoutHandler{
			DB:               db.DB,
			KeyID:            cfg.Razorpay.KeyID,
			KeySecret:        cfg.Razorpay.KeySecret,
			WebhookSecret:    cfg.Razorpay.WebhookSecret,
			SuccessURL:       cfg.Razorpay.SuccessURL,
			CancelURL:        cfg.Razorpay.CancelURL,
			BasePlanID:       cfg.RazorpayPlans.BasePlanID,
			ProPlanID:        cfg.RazorpayPlans.ProPlanID,
			EnterprisePlanID: cfg.RazorpayPlans.EnterprisePlanID,
		}
		billing.POST("/razorpay/order", razorpayHandler.CreateRazorpayOrder)
		billing.POST("/razorpay/verify", razorpayHandler.VerifyRazorpayPayment)
		billing.POST("/razorpay/webhook", razorpayHandler.HandleRazorpayWebhook)
		billing.GET("/razorpay/payments", razorpayHandler.GetRazorpayPayments)
		billing.POST("/razorpay/ubb/verify", razorpayHandler.VerifyUBBOveragePayment)

		// Usage-Based Billing (UBB)
		billing.POST("/ubb/streams", ubbHandler.CreateStream)
		billing.GET("/ubb/streams", ubbHandler.ListStreams)
		billing.DELETE("/ubb/streams/:id", ubbHandler.DeleteStream)
		billing.POST("/ubb/streams/:id/usage", ubbHandler.PostUsage)
		billing.GET("/ubb/streams/:id/usage", ubbHandler.GetUsageSummary)
		billing.POST("/ubb/streams/:id/refresh-sub-item", ubbHandler.RefreshStreamSubItem)
		billing.GET("/ubb/invoice/preview", ubbHandler.PreviewInvoice)
		billing.GET("/ubb/invoice/dryrun", ubbHandler.DryRunInvoice)
		billing.POST("/ubb/invoice/pay", ubbHandler.PayUBBInvoice)
		billing.GET("/ubb/subscription-items", ubbHandler.GetSubscriptionItems)
		billing.GET("/ubb/next-bill", ubbHandler.GetNextBillSummary)
		billing.POST("/ubb/revenue/snapshot", ubbHandler.SnapshotRevenue)
	}

	// Start server
	log.Println("Billing Service starting on port 8082")
	if err := router.Run(":8082"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
