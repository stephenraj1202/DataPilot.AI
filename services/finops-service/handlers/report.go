package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ─── Public types ─────────────────────────────────────────────────────────────

type ReportHandler struct {
	DB        *sql.DB
	EmailCfg  EmailConfig
	AESKey    string
	Scheduler *SyncScheduler // used to trigger live sync before building report
}

type ReportScheduler struct {
	DB        *sql.DB
	EmailCfg  EmailConfig
	AESKey    string
	Scheduler *SyncScheduler
}

type smtpResolvedCfg struct {
	Host, Port, Username, Password, From string
	UseTLS                               bool
}

// ─── Schedule request / response ─────────────────────────────────────────────

type reportScheduleRequest struct {
	Name       string   `json:"name"        binding:"required"`
	Frequency  string   `json:"frequency"   binding:"required,oneof=daily weekly monthly"`
	DayOfWeek  *int     `json:"day_of_week"`
	DayOfMonth *int     `json:"day_of_month"`
	SendHour   int      `json:"send_hour"`
	Recipients []string `json:"recipients"  binding:"required,min=1"`
	ReportType string   `json:"report_type" binding:"required,oneof=cost_summary anomalies recommendations full"`
	IsActive   bool     `json:"is_active"`
}

type reportScheduleRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Frequency  string   `json:"frequency"`
	DayOfWeek  *int     `json:"day_of_week"`
	DayOfMonth *int     `json:"day_of_month"`
	SendHour   int      `json:"send_hour"`
	Recipients []string `json:"recipients"`
	ReportType string   `json:"report_type"`
	IsActive   bool     `json:"is_active"`
	LastSentAt *string  `json:"last_sent_at"`
	NextRunAt  string   `json:"next_run_at"`
	CreatedAt  string   `json:"created_at"`
}

// ─── Internal report data structures ─────────────────────────────────────────

type reportData struct {
	Now           time.Time
	MonthLabel    string
	TotalCost     float64
	Forecast      float64
	Providers     []providerSummary
	Services      []reportService
	DailyCosts    []reportDailyCost
	Anomalies     []reportAnomaly
	Recs          []reportRec
	TotalSaves    float64
	AccountGroups []reportAccountGroup
}

type providerSummary struct {
	Name     string
	Cost     float64
	Forecast float64
}

type reportService struct {
	Name     string
	Provider string
	Cost     float64
}

type reportDailyCost struct {
	Date string
	Cost float64
}

type reportAnomaly struct {
	Date, Severity, AccountName string
	Actual, Baseline, Deviation float64
}

type reportRec struct {
	Type, Service, Desc, AccountName string
	Savings                          float64
}

type reportAccountGroup struct {
	AccountName string
	Provider    string
	LastSyncAt  string
	LastSyncOK  bool
	RegionCount int
	MTDCost     float64
	Forecast    float64
	Tiles       []reportTile
	Services    []reportService
}

type reportTile struct {
	Icon, Label, Color string
	Count              int
}

// ─── HTTP handlers ────────────────────────────────────────────────────────────

func (h *ReportHandler) CreateSchedule(c *gin.Context) {
	accountID, userID := c.GetString("account_id"), c.GetString("user_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req reportScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rj, _ := json.Marshal(req.Recipients)
	nextRun := calcNextRun(req.Frequency, req.DayOfWeek, req.DayOfMonth, req.SendHour)
	id := uuid.New().String()
	_, err := h.DB.Exec(
		`INSERT INTO report_schedules
		 (id,account_id,created_by,name,frequency,day_of_week,day_of_month,
		  send_hour,recipients,report_type,is_active,next_run_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, accountID, userID, req.Name, req.Frequency,
		req.DayOfWeek, req.DayOfMonth, req.SendHour,
		string(rj), req.ReportType, req.IsActive, nextRun,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create schedule"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "next_run_at": nextRun.Format(time.RFC3339)})
}

func (h *ReportHandler) ListSchedules(c *gin.Context) {
	accountID := c.GetString("account_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	rows, err := h.DB.Query(
		`SELECT id,name,frequency,day_of_week,day_of_month,send_hour,
		        recipients,report_type,is_active,last_sent_at,next_run_at,created_at
		 FROM report_schedules WHERE account_id=? AND deleted_at IS NULL ORDER BY created_at DESC`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query schedules"})
		return
	}
	defer rows.Close()
	var list []reportScheduleRow
	for rows.Next() {
		var s reportScheduleRow
		var rj []byte
		if err := rows.Scan(&s.ID, &s.Name, &s.Frequency, &s.DayOfWeek, &s.DayOfMonth,
			&s.SendHour, &rj, &s.ReportType, &s.IsActive, &s.LastSentAt, &s.NextRunAt, &s.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(rj, &s.Recipients)
		list = append(list, s)
	}
	c.JSON(http.StatusOK, gin.H{"schedules": list})
}

func (h *ReportHandler) UpdateSchedule(c *gin.Context) {
	accountID, scheduleID := c.GetString("account_id"), c.Param("id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req reportScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rj, _ := json.Marshal(req.Recipients)
	nextRun := calcNextRun(req.Frequency, req.DayOfWeek, req.DayOfMonth, req.SendHour)
	res, err := h.DB.Exec(
		`UPDATE report_schedules
		 SET name=?,frequency=?,day_of_week=?,day_of_month=?,send_hour=?,
		     recipients=?,report_type=?,is_active=?,next_run_at=?,updated_at=NOW()
		 WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		req.Name, req.Frequency, req.DayOfWeek, req.DayOfMonth, req.SendHour,
		string(rj), req.ReportType, req.IsActive, nextRun, scheduleID, accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update schedule"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule updated"})
}

func (h *ReportHandler) DeleteSchedule(c *gin.Context) {
	accountID, scheduleID := c.GetString("account_id"), c.Param("id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	res, err := h.DB.Exec(
		`UPDATE report_schedules SET deleted_at=NOW() WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		scheduleID, accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete schedule"})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule deleted"})
}

func (h *ReportHandler) SendReportNow(c *gin.Context) {
	accountID := c.GetString("account_id")
	if accountID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		Recipients []string `json:"recipients"  binding:"required,min=1"`
		ReportType string   `json:"report_type" binding:"required,oneof=cost_summary anomalies recommendations full"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	html, err := h.buildReport(accountID, req.ReportType)
	if err != nil {
		log.Printf("[report] build failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build report"})
		return
	}
	cfg := h.resolveAccountSMTP(accountID)
	subj := fmt.Sprintf("[FinOps Report] %s — %s", reportTypeLabel(req.ReportType), time.Now().Format("Jan 2, 2006"))
	if err := h.deliverEmail(cfg, req.Recipients, subj, html); err != nil {
		log.Printf("[report] send failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "report sent", "recipients": req.Recipients})
}

// ─── Scheduler ────────────────────────────────────────────────────────────────

func (rs *ReportScheduler) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rs.runDue()
		}
	}()
}

func (rs *ReportScheduler) runDue() {
	rows, err := rs.DB.Query(
		`SELECT id,account_id,recipients,report_type,frequency,day_of_week,day_of_month,send_hour
		 FROM report_schedules WHERE is_active=TRUE AND deleted_at IS NULL AND next_run_at<=NOW()`,
	)
	if err != nil {
		log.Printf("[report-scheduler] query error: %v", err)
		return
	}
	defer rows.Close()

	type dueRow struct {
		ID, AccountID, ReportType, Frequency string
		Recipients                           []string
		DayOfWeek, DayOfMonth                *int
		SendHour                             int
	}
	var due []dueRow
	for rows.Next() {
		var d dueRow
		var rj []byte
		if err := rows.Scan(&d.ID, &d.AccountID, &rj, &d.ReportType,
			&d.Frequency, &d.DayOfWeek, &d.DayOfMonth, &d.SendHour); err != nil {
			continue
		}
		_ = json.Unmarshal(rj, &d.Recipients)
		due = append(due, d)
	}

	h := &ReportHandler{DB: rs.DB, EmailCfg: rs.EmailCfg, AESKey: rs.AESKey, Scheduler: rs.Scheduler}
	for _, d := range due {
		go func(d dueRow) {
			html, err := h.buildReport(d.AccountID, d.ReportType)
			if err != nil {
				log.Printf("[report-scheduler] build failed for %s: %v", d.ID, err)
				return
			}
			cfg := h.resolveAccountSMTP(d.AccountID)
			subj := fmt.Sprintf("[FinOps Report] %s — %s",
				reportTypeLabel(d.ReportType), time.Now().Format("Jan 2, 2006"))
			if err := h.deliverEmail(cfg, d.Recipients, subj, html); err != nil {
				log.Printf("[report-scheduler] send failed for %s: %v", d.ID, err)
			}
			nextRun := calcNextRun(d.Frequency, d.DayOfWeek, d.DayOfMonth, d.SendHour)
			_, _ = rs.DB.Exec(
				`UPDATE report_schedules SET last_sent_at=NOW(),next_run_at=? WHERE id=?`,
				nextRun, d.ID,
			)
		}(d)
	}
}
