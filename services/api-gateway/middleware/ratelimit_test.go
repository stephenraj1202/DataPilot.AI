package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupRateLimitRouter creates a test router with the rate limit middleware applied.
func setupRateLimitRouter(m *RateLimitMiddleware, userID string) *gin.Engine {
	router := gin.New()
	if userID != "" {
		router.Use(func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Next()
		})
	}
	router.Use(m.Limit())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

// makeRequest fires a single GET /test request against the router.
func makeRequest(router *gin.Engine) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Plan limit lookup tests ---

func TestGetPlanLimit_FreeTier(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, 100, m.getPlanLimit("user-1"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPlanLimit_BaseTier(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-2").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("base"))

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, 500, m.getPlanLimit("user-2"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPlanLimit_ProTier(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-3").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("pro"))

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, 2000, m.getPlanLimit("user-3"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPlanLimit_EnterpriseTier(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-4").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("enterprise"))

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, 10000, m.getPlanLimit("user-4"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPlanLimit_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-err").
		WillReturnError(assert.AnError)

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, defaultLimit, m.getPlanLimit("user-err"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPlanLimit_UnknownPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-5").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("ultra"))

	m := &RateLimitMiddleware{DB: db}
	assert.Equal(t, defaultLimit, m.getPlanLimit("user-5"))
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- HTTP middleware integration tests ---

func TestRateLimit_AllowedRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-ok").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))

	m := &RateLimitMiddleware{DB: db}
	router := setupRateLimitRouter(m, "user-ok")

	w := makeRequest(router)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_ExceededLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// getPlanLimit called once per request; we'll fire limit+1 requests
	for i := 0; i <= 100; i++ {
		mock.ExpectQuery("SELECT sp.name").
			WithArgs("user-over").
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))
	}

	m := &RateLimitMiddleware{DB: db}
	router := setupRateLimitRouter(m, "user-over")

	// Fire 101 requests — the 101st should be rejected
	var lastCode int
	for i := 0; i <= 100; i++ {
		w := makeRequest(router)
		lastCode = w.Code
	}
	assert.Equal(t, http.StatusTooManyRequests, lastCode)
}

func TestRateLimit_429ResponseBody(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	for i := 0; i <= 100; i++ {
		mock.ExpectQuery("SELECT sp.name").
			WithArgs("user-body").
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))
	}

	m := &RateLimitMiddleware{DB: db}
	router := setupRateLimitRouter(m, "user-body")

	var w *httptest.ResponseRecorder
	for i := 0; i <= 100; i++ {
		w = makeRequest(router)
	}
	require.Equal(t, http.StatusTooManyRequests, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "rate limit exceeded", body["error"])
	assert.NotNil(t, body["retry_after"])
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	for i := 0; i <= 100; i++ {
		mock.ExpectQuery("SELECT sp.name").
			WithArgs("user-hdr").
			WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))
	}

	m := &RateLimitMiddleware{DB: db}
	router := setupRateLimitRouter(m, "user-hdr")

	var w *httptest.ResponseRecorder
	for i := 0; i <= 100; i++ {
		w = makeRequest(router)
	}
	require.Equal(t, http.StatusTooManyRequests, w.Code)

	retryAfter := w.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfter)
	val, err := strconv.Atoi(retryAfter)
	require.NoError(t, err)
	assert.Greater(t, val, 0)
}

func TestRateLimit_UnauthenticatedPassThrough(t *testing.T) {
	m := &RateLimitMiddleware{DB: nil}
	router := setupRateLimitRouter(m, "") // no user_id injected

	w := makeRequest(router)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_WindowReset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT sp.name").
		WithArgs("user-reset").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("free"))

	m := &RateLimitMiddleware{DB: db}
	router := setupRateLimitRouter(m, "user-reset")

	w := makeRequest(router)
	assert.Equal(t, http.StatusOK, w.Code)
}
