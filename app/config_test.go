package app

import "testing"

func TestLookupEnvWithDefault(t *testing.T) {
	t.Run("returns fallback when unset", func(t *testing.T) {
		t.Setenv("APP_TEST_DEFAULT_UNSET", "")
		got := lookupEnvWithDefault("APP_TEST_DEFAULT_UNSET", "fallback")
		if got != "fallback" {
			t.Fatalf("expected fallback value, got %q", got)
		}
	})

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("APP_TEST_DEFAULT_SET", "configured")
		got := lookupEnvWithDefault("APP_TEST_DEFAULT_SET", "fallback")
		if got != "configured" {
			t.Fatalf("expected configured value, got %q", got)
		}
	})
}

func TestLookupEnvWithFallback(t *testing.T) {
	t.Run("uses primary when present", func(t *testing.T) {
		t.Setenv("APP_TEST_PRIMARY", "primary")
		t.Setenv("APP_TEST_SECONDARY", "secondary")

		got := lookupEnvWithFallback("APP_TEST_PRIMARY", "APP_TEST_SECONDARY")
		if got != "primary" {
			t.Fatalf("expected primary value, got %q", got)
		}
	})

	t.Run("uses secondary when primary empty", func(t *testing.T) {
		t.Setenv("APP_TEST_PRIMARY_EMPTY", "")
		t.Setenv("APP_TEST_SECONDARY_ONLY", "secondary")

		got := lookupEnvWithFallback("APP_TEST_PRIMARY_EMPTY", "APP_TEST_SECONDARY_ONLY")
		if got != "secondary" {
			t.Fatalf("expected secondary value, got %q", got)
		}
	})

	t.Run("returns empty when both missing", func(t *testing.T) {
		t.Setenv("APP_TEST_PRIMARY_MISSING", "")
		t.Setenv("APP_TEST_SECONDARY_MISSING", "")

		got := lookupEnvWithFallback("APP_TEST_PRIMARY_MISSING", "APP_TEST_SECONDARY_MISSING")
		if got != "" {
			t.Fatalf("expected empty value, got %q", got)
		}
	})
}
