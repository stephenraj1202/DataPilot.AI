package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	stripe "github.com/stripe/stripe-go/v76"
	stripeinvoice "github.com/stripe/stripe-go/v76/invoice"
)

// InvoiceHandler handles invoice listing and PDF download.
type InvoiceHandler struct {
	DB       *sql.DB
	EmailCfg EmailConfig
}

type invoiceResponse struct {
	ID              string `json:"id"`
	StripeInvoiceID string `json:"stripe_invoice_id"`
	AmountCents     int64  `json:"amount_cents"`
	Currency        string `json:"currency"`
	Status          string `json:"status"`
	InvoicePDFURL   string `json:"invoice_pdf_url"`
	CreatedAt       string `json:"created_at"`
}

// ListInvoices returns all invoices for an account.
// GET /billing/invoices
func (h *InvoiceHandler) ListInvoices(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		accountID = c.Query("account_id")
	}
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, stripe_invoice_id, amount_cents, currency, status, COALESCE(invoice_pdf_url,''), created_at
		 FROM stripe_invoices
		 WHERE account_id = ?
		 ORDER BY created_at DESC`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	defer rows.Close()

	var invoices []invoiceResponse
	for rows.Next() {
		var inv invoiceResponse
		if err := rows.Scan(&inv.ID, &inv.StripeInvoiceID, &inv.AmountCents, &inv.Currency, &inv.Status, &inv.InvoicePDFURL, &inv.CreatedAt); err != nil {
			continue
		}
		invoices = append(invoices, inv)
	}

	if invoices == nil {
		invoices = []invoiceResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"invoices": invoices})
}

// DownloadInvoicePDF proxies the Stripe invoice PDF download.
// GET /billing/invoices/:id/pdf
func (h *InvoiceHandler) DownloadInvoicePDF(c *gin.Context) {
	invoiceID := c.Param("id")

	// Look up the stripe_invoice_id and pdf URL
	var stripeInvoiceID, pdfURL string
	err := h.DB.QueryRow(
		`SELECT stripe_invoice_id, COALESCE(invoice_pdf_url,'') FROM stripe_invoices WHERE id = ?`,
		invoiceID,
	).Scan(&stripeInvoiceID, &pdfURL)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "invoice not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// If we have a direct PDF URL, redirect to it
	if pdfURL != "" {
		c.Redirect(http.StatusFound, pdfURL)
		return
	}

	// Otherwise fetch from Stripe API
	if isLocalSubID(stripeInvoiceID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "PDF not available for this invoice"})
		return
	}

	inv, err := stripeinvoice.Get(stripeInvoiceID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve invoice from Stripe"})
		return
	}

	if inv.InvoicePDF == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "PDF not available"})
		return
	}

	// Update the PDF URL in the database
	_, _ = h.DB.Exec(
		`UPDATE stripe_invoices SET invoice_pdf_url=? WHERE id=?`,
		inv.InvoicePDF, invoiceID,
	)

	c.Redirect(http.StatusFound, inv.InvoicePDF)
}

// SendInvoiceEmail sends an invoice notification email with PDF link.
func (h *InvoiceHandler) SendInvoiceEmail(accountID string, inv *stripe.Invoice) {
	// Get Account_Owner email
	var ownerEmail string
	_ = h.DB.QueryRow(
		`SELECT u.email FROM users u
		 JOIN user_roles ur ON ur.user_id = u.id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE u.account_id = ? AND r.name = 'account_owner'
		 LIMIT 1`,
		accountID,
	).Scan(&ownerEmail)

	if ownerEmail == "" {
		return
	}

	subject := fmt.Sprintf("Invoice #%s - FinOps Platform", inv.Number)
	body := fmt.Sprintf(`
Your invoice is ready.

Invoice Number: %s
Amount: %s %.2f
Status: %s

`, inv.Number, string(inv.Currency), float64(inv.AmountDue)/100.0, string(inv.Status))

	if inv.InvoicePDF != "" {
		body += fmt.Sprintf("Download PDF: %s\n", inv.InvoicePDF)
	}

	body += "\nBest regards,\nFinOps Platform Team\n"

	recipients := []string{ownerEmail}
	if h.EmailCfg.SuperAdminEmail != "" {
		recipients = append(recipients, h.EmailCfg.SuperAdminEmail)
	}

	if err := sendEmail(h.EmailCfg, recipients, subject, body); err != nil {
		log.Printf("Failed to send invoice email for account %s: %v", accountID, err)
	}
}

// fetchPDFBytes downloads the PDF from a URL and returns the bytes.
func fetchPDFBytes(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
