package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Role IDs matching migrations/009_seed_roles_and_permissions.sql
const (
	roleIDSuperAdmin   = "00000000-0000-0000-0000-000000000001"
	roleIDAccountOwner = "00000000-0000-0000-0000-000000000002"
	roleIDAdmin        = "00000000-0000-0000-0000-000000000003"
	roleIDUser         = "00000000-0000-0000-0000-000000000004"
	roleIDViewer       = "00000000-0000-0000-0000-000000000005"

	testUserID = "user-test-0000-0000-000000000001"
)

// --- HasPermission unit tests ---

// TestHasPermission_SuperAdminBypass verifies Requirement 2.2:
// Super_Admin bypasses all permission checks and always gets access.
func TestHasPermission_SuperAdminBypass(t *testing.T) {
	permissions := []string{
		"finops:read", "finops:write", "finops:delete", "finops:execute",
		"query:read", "query:write", "query:delete", "query:execute",
		"billing:read", "billing:write", "billing:delete", "billing:manage",
		"settings:read", "settings:write", "settings:delete", "settings:manage",
	}

	for _, perm := range permissions {
		t.Run(perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Super_Admin check returns 1 — bypass triggered
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.True(t, allowed, "super_admin must be allowed for permission %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestHasPermission_SuperAdminBypass_NoSecondQuery verifies that when the user is
// Super_Admin the second (role_permissions JOIN) query is never executed.
func TestHasPermission_SuperAdminBypass_NoSecondQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// No second query expectation — if it fires, ExpectationsWereMet will catch extra calls
	allowed, err := HasPermission(db, testUserID, "billing:delete")
	require.NoError(t, err)
	assert.True(t, allowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestHasPermission_AccountOwner verifies Requirement 2.3:
// Account_Owner has access to all modules except Super_Admin functions.
func TestHasPermission_AccountOwner(t *testing.T) {
	allowedPerms := []string{
		"finops:read", "finops:write", "finops:delete", "finops:execute",
		"query:read", "query:write", "query:delete", "query:execute",
		"billing:read", "billing:write", "billing:manage",
		"settings:read", "settings:write", "settings:delete", "settings:manage",
	}

	for _, perm := range allowedPerms {
		t.Run("allowed_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Not super_admin
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			// Has the permission via account_owner role
			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.True(t, allowed, "account_owner must be allowed for %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	// billing:delete is super_admin-only; account_owner does NOT have it
	t.Run("denied_billing:delete", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
			WithArgs(testUserID, superAdminRoleID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		mock.ExpectQuery(`SELECT COUNT\(\*\)`).
			WithArgs(testUserID, "billing:delete").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		allowed, err := HasPermission(db, testUserID, "billing:delete")
		require.NoError(t, err)
		assert.False(t, allowed, "account_owner must NOT have billing:delete")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// TestHasPermission_Admin verifies Requirement 2.4:
// Admin has read-write access to FinOps_Service, AI_Query_Engine, and database connectors.
func TestHasPermission_Admin(t *testing.T) {
	allowedPerms := []string{
		"finops:read", "finops:write",
		"query:read", "query:write", "query:execute",
		"billing:read",
		"settings:read", "settings:write",
	}
	deniedPerms := []string{
		"finops:delete", "finops:execute",
		"query:delete",
		"billing:write", "billing:delete", "billing:manage",
		"settings:delete", "settings:manage",
	}

	for _, perm := range allowedPerms {
		t.Run("allowed_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.True(t, allowed, "admin must be allowed for %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	for _, perm := range deniedPerms {
		t.Run("denied_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.False(t, allowed, "admin must NOT have %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestHasPermission_User verifies Requirement 2.5:
// User has read-write access to AI_Query_Engine and read-only access to FinOps_Service.
func TestHasPermission_User(t *testing.T) {
	allowedPerms := []string{
		"finops:read",
		"query:read", "query:write", "query:execute",
	}
	deniedPerms := []string{
		"finops:write", "finops:delete", "finops:execute",
		"query:delete",
		"billing:read", "billing:write", "billing:delete", "billing:manage",
		"settings:read", "settings:write", "settings:delete", "settings:manage",
	}

	for _, perm := range allowedPerms {
		t.Run("allowed_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.True(t, allowed, "user must be allowed for %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	for _, perm := range deniedPerms {
		t.Run("denied_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.False(t, allowed, "user must NOT have %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestHasPermission_Viewer verifies Requirement 2.6:
// Viewer has read-only access to all dashboards.
func TestHasPermission_Viewer(t *testing.T) {
	allowedPerms := []string{
		"finops:read",
		"query:read",
		"billing:read",
		"settings:read",
	}
	deniedPerms := []string{
		"finops:write", "finops:delete", "finops:execute",
		"query:write", "query:delete", "query:execute",
		"billing:write", "billing:delete", "billing:manage",
		"settings:write", "settings:delete", "settings:manage",
	}

	for _, perm := range allowedPerms {
		t.Run("allowed_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.True(t, allowed, "viewer must be allowed for %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	for _, perm := range deniedPerms {
		t.Run("denied_"+perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			allowed, err := HasPermission(db, testUserID, perm)
			require.NoError(t, err)
			assert.False(t, allowed, "viewer must NOT have %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestHasPermission_DBError verifies that a database error is propagated correctly.
func TestHasPermission_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnError(assert.AnError)

	allowed, err := HasPermission(db, testUserID, "finops:read")
	assert.Error(t, err)
	assert.False(t, allowed)
}

// --- RequirePermission middleware tests ---

// TestRequirePermission_Allowed verifies that an authorized user reaches the handler.
func TestRequirePermission_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs(testUserID, "finops:read").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
	router.GET("/test", RequirePermission(db, "finops:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRequirePermission_Forbidden verifies that an unauthorized user gets HTTP 403.
func TestRequirePermission_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs(testUserID, "billing:delete").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
	router.GET("/test", RequirePermission(db, "billing:delete"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "forbidden")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRequirePermission_MissingUserID verifies HTTP 401 when user_id is absent from context.
func TestRequirePermission_MissingUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	router := gin.New()
	// No user_id set in context
	router.GET("/test", RequirePermission(db, "finops:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequirePermission_SuperAdminAllowed verifies Requirement 2.2:
// Super_Admin middleware passes for any permission without a second DB query.
func TestRequirePermission_SuperAdminAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Only the super_admin check fires; no role_permissions query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
	router.DELETE("/test", RequirePermission(db, "billing:delete"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("DELETE", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRequirePermission_ViewerDeniedWrite verifies Requirement 2.6:
// Viewer cannot perform write operations.
func TestRequirePermission_ViewerDeniedWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	writePerms := []string{"finops:write", "query:write", "settings:write", "billing:write"}

	for _, perm := range writePerms {
		t.Run(perm, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
				WithArgs(testUserID, superAdminRoleID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			mock.ExpectQuery(`SELECT COUNT\(\*\)`).
				WithArgs(testUserID, perm).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
			router.POST("/test", RequirePermission(db, perm), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("POST", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code, "viewer must be denied %q", perm)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRequirePermission_UserDeniedFinOpsWrite verifies Requirement 2.5:
// User role cannot write to FinOps_Service.
func TestRequirePermission_UserDeniedFinOpsWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs(testUserID, "finops:write").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
	router.POST("/test", RequirePermission(db, "finops:write"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestRequirePermission_DBError returns HTTP 500 when the DB query fails.
func TestRequirePermission_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_roles WHERE user_id = \? AND role_id = \?`).
		WithArgs(testUserID, superAdminRoleID).
		WillReturnError(assert.AnError)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", testUserID); c.Next() })
	router.GET("/test", RequirePermission(db, "finops:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
