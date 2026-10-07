package db

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ValidateRecoveryTestDSN checks the parsed database, not a substring of the
// connection string. A URL query parameter can override its path database.
func ValidateRecoveryTestDSN(dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("invalid recovery test connection")
	}
	if !strings.HasPrefix(cfg.Database, "educate_recovery_test_") {
		return fmt.Errorf("recovery tests require a dedicated educate_recovery_test_ database")
	}
	return nil
}
