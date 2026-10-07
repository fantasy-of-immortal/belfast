package db

import "testing"

func TestRecoveryTestDSNUsesActualDatabase(t *testing.T) {
	for _, dsn := range []string{
		"postgres://localhost/educate_recovery_test_a",
		"host=localhost dbname=educate_recovery_test_a",
	} {
		if err := ValidateRecoveryTestDSN(dsn); err != nil {
			t.Fatal(err)
		}
	}
	for _, dsn := range []string{
		"postgres://localhost/belfast?application_name=/educate_recovery_test_a",
		"postgres://localhost/educate_recovery_test_a?dbname=belfast",
		"host=localhost dbname=belfast application_name=educate_recovery_test_a",
		"postgres://localhost/belfast",
	} {
		if err := ValidateRecoveryTestDSN(dsn); err == nil {
			t.Fatal("unsafe test target accepted", dsn)
		}
	}
}
