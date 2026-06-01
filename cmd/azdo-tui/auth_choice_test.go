package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Elpulgo/azdo/internal/config"
)

func TestSelectAuthMethod_PAT(t *testing.T) {
	r := strings.NewReader("1\n")
	var w strings.Builder
	method, err := selectAuthMethod(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "pat" {
		t.Errorf("got %q, want %q", method, "pat")
	}
}

func TestSelectAuthMethod_AzCLI(t *testing.T) {
	r := strings.NewReader("2\n")
	var w strings.Builder
	method, err := selectAuthMethod(r, &w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method != "az-cli" {
		t.Errorf("got %q, want %q", method, "az-cli")
	}
}

func TestSelectAuthMethod_InvalidChoice(t *testing.T) {
	r := strings.NewReader("3\n")
	var w strings.Builder
	_, err := selectAuthMethod(r, &w)
	if err == nil {
		t.Error("expected error for invalid choice, got nil")
	}
}

func TestSelectAuthMethod_EmptyInput(t *testing.T) {
	r := strings.NewReader("\n")
	var w strings.Builder
	_, err := selectAuthMethod(r, &w)
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestUpdateConfigAuthMethod_UpdatesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := "organization: myorg\nprojects:\n  - myproject\npolling_interval: 60\ntheme: dark\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	updated, err := updateConfigAuthMethod(configPath, "az-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected updated=true for existing config")
	}

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.AuthMethod != "az-cli" {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, "az-cli")
	}
}

func TestUpdateConfigAuthMethod_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "nonexistent", "config.yaml")

	updated, err := updateConfigAuthMethod(configPath, "az-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Error("expected updated=false when config doesn't exist")
	}
}
