package config_test

import (
	"acuity/pkg/config"
	"os"
	"testing"
)

func TestConfigDefaultsAndEnv(t *testing.T) {
	// Set custom environment variable
	_ = os.Setenv("PORT", "4567")
	_ = os.Setenv("DEBUG", "true")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("DEBUG")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if cfg.Port != "4567" {
		t.Errorf("Expected Port to be '4567', got '%s'", cfg.Port)
	}
	if !cfg.Debug {
		t.Errorf("Expected Debug to be true, got %v", cfg.Debug)
	}
	if cfg.ImagesPerPage <= 0 {
		t.Errorf("Expected positive ImagesPerPage default, got %d", cfg.ImagesPerPage)
	}
	if cfg.DBPath == "" {
		t.Errorf("Expected non-empty default DBPath")
	}
}
