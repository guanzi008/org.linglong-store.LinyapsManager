package main

import "testing"

func TestParseKillArgs(t *testing.T) {
	t.Parallel()

	t.Run("no-signal", func(t *testing.T) {
		appID, sig, err := parseKillArgs([]string{"org.example.App"})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if appID != "org.example.App" {
			t.Fatalf("expected appID org.example.App, got %q", appID)
		}
		if sig != "" {
			t.Fatalf("expected empty signal, got %q", sig)
		}
	})

	t.Run("short-signal", func(t *testing.T) {
		appID, sig, err := parseKillArgs([]string{"-s", "SIGKILL", "org.example.App"})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if appID != "org.example.App" {
			t.Fatalf("expected appID org.example.App, got %q", appID)
		}
		if sig != "SIGKILL" {
			t.Fatalf("expected SIGKILL, got %q", sig)
		}
	})

	t.Run("long-signal", func(t *testing.T) {
		appID, sig, err := parseKillArgs([]string{"--signal", "SIGTERM", "org.example.App"})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if appID != "org.example.App" {
			t.Fatalf("expected appID org.example.App, got %q", appID)
		}
		if sig != "SIGTERM" {
			t.Fatalf("expected SIGTERM, got %q", sig)
		}
	})

	t.Run("equals-signal", func(t *testing.T) {
		appID, sig, err := parseKillArgs([]string{"--signal=SIGUSR1", "org.example.App"})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if appID != "org.example.App" {
			t.Fatalf("expected appID org.example.App, got %q", appID)
		}
		if sig != "SIGUSR1" {
			t.Fatalf("expected SIGUSR1, got %q", sig)
		}
	})

	t.Run("missing-app", func(t *testing.T) {
		_, _, err := parseKillArgs([]string{"-s", "SIGKILL"})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("unknown-flag", func(t *testing.T) {
		_, _, err := parseKillArgs([]string{"--nope", "org.example.App"})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("extra-arg", func(t *testing.T) {
		_, _, err := parseKillArgs([]string{"org.example.App", "extra"})
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
