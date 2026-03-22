package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	username := getEnv("DB_USERNAME", "root")
	password := getEnv("DB_PASSWORD", "MySQL123$$")
	dbName := getEnv("DB_NAME", "finops_platform")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		username, password, host, port, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database successfully")

	// Create migrations tracking table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	if err != nil {
		log.Fatalf("Failed to create schema_migrations table: %v", err)
	}

	files, err := filepath.Glob("*.sql")
	if err != nil {
		log.Fatalf("Failed to read migration files: %v", err)
	}
	sort.Strings(files)

	for _, file := range files {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", file).Scan(&count)
		if err != nil {
			log.Fatalf("Failed to check migration status for %s: %v", file, err)
		}
		if count > 0 {
			log.Printf("Skipping already-applied migration: %s", file)
			continue
		}

		log.Printf("Running migration: %s", file)
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", file, err)
		}

		if err := execStatements(db, string(content)); err != nil {
			log.Fatalf("Failed to execute migration %s: %v", file, err)
		}

		_, err = db.Exec("INSERT INTO schema_migrations (name) VALUES (?)", file)
		if err != nil {
			log.Fatalf("Failed to record migration %s: %v", file, err)
		}

		log.Printf("Successfully executed migration: %s", file)
	}

	log.Println("All migrations completed successfully")
}

// execStatements splits SQL content on semicolons and executes each statement
// individually, ignoring benign errors like duplicate columns or existing indexes.
func execStatements(db *sql.DB, content string) error {
	stmts := splitSQL(content)
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		_, err := db.Exec(stmt)
		if err != nil {
			if isBenign(err) {
				log.Printf("  Skipping (already exists): %s", shortStmt(stmt))
				continue
			}
			return fmt.Errorf("%w\nStatement: %s", err, shortStmt(stmt))
		}
	}
	return nil
}

// isBenign returns true for errors we can safely ignore during migrations.
func isBenign(err error) bool {
	me, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}
	switch me.Number {
	case 1060: // Duplicate column name
		return true
	case 1061: // Duplicate key name (index already exists)
		return true
	case 1050: // Table already exists (belt-and-suspenders)
		return true
	}
	return false
}

// splitSQL splits on semicolons, respecting that multiStatements driver
// would normally handle this — but we want per-statement error handling.
func splitSQL(content string) []string {
	var stmts []string
	var cur strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue // skip comment lines
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmts = append(stmts, strings.TrimSuffix(strings.TrimSpace(cur.String()), ";"))
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

func shortStmt(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
