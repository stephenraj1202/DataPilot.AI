// seed_rbac.go seeds the roles, permissions, and role_permissions tables.
// It can be run standalone:
//
//	DB_HOST=localhost DB_PASSWORD=secret go run seed_rbac.go
//
// or called programmatically via SeedRBAC(db).
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// Role represents a platform role.
type Role struct {
	ID          string
	Name        string
	Description string
}

// Permission represents a resource:action permission.
type Permission struct {
	ID       string
	Name     string
	Resource string
	Action   string
}

var roles = []Role{
	{"00000000-0000-0000-0000-000000000001", "super_admin", "Platform owner with global access to all accounts and billing"},
	{"00000000-0000-0000-0000-000000000002", "account_owner", "Primary user who owns an Account subscription; all modules except super_admin"},
	{"00000000-0000-0000-0000-000000000003", "admin", "Read-write access to FinOps, AI Query Engine, and database connectors"},
	{"00000000-0000-0000-0000-000000000004", "user", "Read-write access to AI Query Engine and read-only access to FinOps"},
	{"00000000-0000-0000-0000-000000000005", "viewer", "Read-only access to all dashboards"},
}

var permissions = []Permission{
	// finops
	{"10000000-0000-0000-0000-000000000001", "finops:read", "finops", "read"},
	{"10000000-0000-0000-0000-000000000002", "finops:write", "finops", "write"},
	{"10000000-0000-0000-0000-000000000003", "finops:delete", "finops", "delete"},
	{"10000000-0000-0000-0000-000000000004", "finops:execute", "finops", "execute"},
	// query
	{"10000000-0000-0000-0000-000000000005", "query:read", "query", "read"},
	{"10000000-0000-0000-0000-000000000006", "query:write", "query", "write"},
	{"10000000-0000-0000-0000-000000000007", "query:delete", "query", "delete"},
	{"10000000-0000-0000-0000-000000000008", "query:execute", "query", "execute"},
	// billing
	{"10000000-0000-0000-0000-000000000009", "billing:read", "billing", "read"},
	{"10000000-0000-0000-0000-000000000010", "billing:write", "billing", "write"},
	{"10000000-0000-0000-0000-000000000011", "billing:delete", "billing", "delete"},
	{"10000000-0000-0000-0000-000000000012", "billing:manage", "billing", "manage"},
	// settings
	{"10000000-0000-0000-0000-000000000013", "settings:read", "settings", "read"},
	{"10000000-0000-0000-0000-000000000014", "settings:write", "settings", "write"},
	{"10000000-0000-0000-0000-000000000015", "settings:delete", "settings", "delete"},
	{"10000000-0000-0000-0000-000000000016", "settings:manage", "settings", "manage"},
}

// rolePermissions maps role ID → slice of permission IDs.
// super_admin gets all permissions (handled dynamically).
var rolePermissions = map[string][]string{
	// account_owner: all except billing:delete
	"00000000-0000-0000-0000-000000000002": {
		"10000000-0000-0000-0000-000000000001", // finops:read
		"10000000-0000-0000-0000-000000000002", // finops:write
		"10000000-0000-0000-0000-000000000003", // finops:delete
		"10000000-0000-0000-0000-000000000004", // finops:execute
		"10000000-0000-0000-0000-000000000005", // query:read
		"10000000-0000-0000-0000-000000000006", // query:write
		"10000000-0000-0000-0000-000000000007", // query:delete
		"10000000-0000-0000-0000-000000000008", // query:execute
		"10000000-0000-0000-0000-000000000009", // billing:read
		"10000000-0000-0000-0000-000000000010", // billing:write
		"10000000-0000-0000-0000-000000000012", // billing:manage
		"10000000-0000-0000-0000-000000000013", // settings:read
		"10000000-0000-0000-0000-000000000014", // settings:write
		"10000000-0000-0000-0000-000000000015", // settings:delete
		"10000000-0000-0000-0000-000000000016", // settings:manage
	},
	// admin: finops:read/write, query:read/write/execute, billing:read, settings:read/write
	"00000000-0000-0000-0000-000000000003": {
		"10000000-0000-0000-0000-000000000001", // finops:read
		"10000000-0000-0000-0000-000000000002", // finops:write
		"10000000-0000-0000-0000-000000000005", // query:read
		"10000000-0000-0000-0000-000000000006", // query:write
		"10000000-0000-0000-0000-000000000008", // query:execute
		"10000000-0000-0000-0000-000000000009", // billing:read
		"10000000-0000-0000-0000-000000000013", // settings:read
		"10000000-0000-0000-0000-000000000014", // settings:write
	},
	// user: finops:read, query:read/write/execute
	"00000000-0000-0000-0000-000000000004": {
		"10000000-0000-0000-0000-000000000001", // finops:read
		"10000000-0000-0000-0000-000000000005", // query:read
		"10000000-0000-0000-0000-000000000006", // query:write
		"10000000-0000-0000-0000-000000000008", // query:execute
	},
	// viewer: finops:read, query:read, billing:read, settings:read
	"00000000-0000-0000-0000-000000000005": {
		"10000000-0000-0000-0000-000000000001", // finops:read
		"10000000-0000-0000-0000-000000000005", // query:read
		"10000000-0000-0000-0000-000000000009", // billing:read
		"10000000-0000-0000-0000-000000000013", // settings:read
	},
}

// SeedRBAC inserts roles, permissions, and role_permissions into the database.
// It is idempotent: existing rows are skipped via INSERT IGNORE.
func SeedRBAC(db *sql.DB) error {
	// Seed roles
	for _, r := range roles {
		_, err := db.Exec(
			`INSERT IGNORE INTO roles (id, name, description, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`,
			r.ID, r.Name, r.Description,
		)
		if err != nil {
			return fmt.Errorf("insert role %s: %w", r.Name, err)
		}
	}
	log.Printf("Seeded %d roles", len(roles))

	// Seed permissions
	for _, p := range permissions {
		_, err := db.Exec(
			`INSERT IGNORE INTO permissions (id, name, resource, action, created_at) VALUES (?, ?, ?, ?, NOW())`,
			p.ID, p.Name, p.Resource, p.Action,
		)
		if err != nil {
			return fmt.Errorf("insert permission %s: %w", p.Name, err)
		}
	}
	log.Printf("Seeded %d permissions", len(permissions))

	// super_admin gets every permission
	superAdminID := "00000000-0000-0000-0000-000000000001"
	for _, p := range permissions {
		_, err := db.Exec(
			`INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)`,
			superAdminID, p.ID,
		)
		if err != nil {
			return fmt.Errorf("map super_admin → %s: %w", p.Name, err)
		}
	}

	// Seed remaining role→permission mappings
	for roleID, permIDs := range rolePermissions {
		for _, permID := range permIDs {
			_, err := db.Exec(
				`INSERT IGNORE INTO role_permissions (role_id, permission_id) VALUES (?, ?)`,
				roleID, permID,
			)
			if err != nil {
				return fmt.Errorf("map role %s → permission %s: %w", roleID, permID, err)
			}
		}
	}
	log.Println("Seeded role_permissions mappings")

	return nil
}

// SeedRBACStandalone is the standalone entry point.
// Call it from main() or run this file with `go run seed_rbac.go migrate.go`.
func SeedRBACStandalone() {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	username := getEnv("DB_USERNAME", "root")
	password := getEnv("DB_PASSWORD", "rootpassword")
	dbName := getEnv("DB_NAME", "finops_platform")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		username, password, host, port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	if err := SeedRBAC(db); err != nil {
		log.Fatalf("RBAC seeding failed: %v", err)
	}

	log.Println("RBAC seeding completed successfully")
}
