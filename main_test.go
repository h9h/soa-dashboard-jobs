package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soa-dashboard-jobs/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		JobPath:   "C:/Dashboard",
		ModelPath: "C:/DashboardModel",
		Port:      "4000",
		Extra: map[string]string{
			config.KeyJobPath:   "C:/Dashboard",
			config.KeyModelPath: "C:/DashboardModel",
			config.KeyPort:      "4000",
		},
	}
}

func TestApplyPortArgumentOverridesConfiguredPort(t *testing.T) {
	cfg := testConfig()

	if err := applyPortArgument(cfg, []string{"4001"}); err != nil {
		t.Fatalf("applyPortArgument: %v", err)
	}
	if cfg.Port != "4001" || cfg.Extra[config.KeyPort] != "4001" {
		t.Errorf("Port = %q / %q, erwartet 4001", cfg.Port, cfg.Extra[config.KeyPort])
	}
}

func TestApplyPortArgumentWithoutArgumentKeepsConfiguredPort(t *testing.T) {
	cfg := testConfig()

	if err := applyPortArgument(cfg, nil); err != nil {
		t.Fatalf("applyPortArgument: %v", err)
	}
	if cfg.Port != "4000" {
		t.Errorf("Port = %q, erwartet 4000", cfg.Port)
	}
}

func TestApplyPortArgumentRejectsNonNumeric(t *testing.T) {
	if err := applyPortArgument(testConfig(), []string{"vier"}); err == nil {
		t.Fatal("ein nicht numerischer Port muss abgelehnt werden")
	}
}

func TestEnsureDirCreatesMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "neu", "tiefer")

	created, err := ensureDir(dir)
	if err != nil {
		t.Fatalf("ensureDir: %v", err)
	}
	if !created {
		t.Error("created = false, erwartet true")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("Verzeichnis wurde nicht angelegt: %v", err)
	}
}

func TestEnsureDirOnExistingDirectoryReportsNotCreated(t *testing.T) {
	dir := t.TempDir()

	created, err := ensureDir(dir)
	if err != nil {
		t.Fatalf("ensureDir: %v", err)
	}
	if created {
		t.Error("created = true, erwartet false")
	}
}

func TestHelpTextMentionsPortsPathsAndRoutes(t *testing.T) {
	text := helpText(testConfig(), true)

	for _, fragment := range []string{
		"http://localhost:4000",
		"C:/Dashboard",
		"C:/DashboardModel",
		"(neu angelegt)",
		"GET  /jobs",
		"POST /job/save",
		"PUT  /log",
		"GET  /checkalive",
	} {
		if !strings.Contains(text, fragment) {
			t.Errorf("Hilfetext enthaelt %q nicht:\n%s", fragment, text)
		}
	}
}

func TestHelpTextWithoutNewDirectory(t *testing.T) {
	text := helpText(testConfig(), false)

	if strings.Contains(text, "(neu angelegt)") {
		t.Error("Hilfetext meldet faelschlich ein neu angelegtes Verzeichnis")
	}
}
