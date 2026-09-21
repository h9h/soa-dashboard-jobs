package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfig legt eine Konfigurationsdatei im Temp-Verzeichnis des Tests an.
func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "jobs.config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Konfigurationsdatei konnte nicht geschrieben werden: %v", err)
	}
	return path
}

func TestLoadReadsAllKeys(t *testing.T) {
	path := writeConfig(t, `{
		"JOB_PATH": "C:/Dashboard",
		"MODEL_PATH": "C:/DashboardModel",
		"LOCAL_SERVER_PORT": "4000",
		"QUEUE_MAP_URL": "http://example.invalid/map.json"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.JobPath != "C:/Dashboard" {
		t.Errorf("JobPath = %q, erwartet %q", cfg.JobPath, "C:/Dashboard")
	}
	if cfg.ModelPath != "C:/DashboardModel" {
		t.Errorf("ModelPath = %q, erwartet %q", cfg.ModelPath, "C:/DashboardModel")
	}
	if cfg.Port != "4000" {
		t.Errorf("Port = %q, erwartet %q", cfg.Port, "4000")
	}
	if got := cfg.Extra["QUEUE_MAP_URL"]; got != "http://example.invalid/map.json" {
		t.Errorf("Extra[QUEUE_MAP_URL] = %q", got)
	}
	if got := cfg.Extra[KeyJobPath]; got != "C:/Dashboard" {
		t.Errorf("Extra enthaelt die Pflichtschluessel nicht: %q", got)
	}
}

func TestLoadAppliesEnvironmentOverride(t *testing.T) {
	path := writeConfig(t, `{"JOB_PATH":"C:/Dashboard","MODEL_PATH":"C:/Model","LOCAL_SERVER_PORT":"4000"}`)

	t.Setenv(EnvPrefix+KeyJobPath, "D:/Jobs")
	t.Setenv(EnvPrefix+"QUEUE_MAP_URL", "http://override.invalid/map.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.JobPath != "D:/Jobs" {
		t.Errorf("JobPath = %q, erwartet die Umgebungsvariable", cfg.JobPath)
	}
	if got := cfg.Extra[KeyJobPath]; got != "D:/Jobs" {
		t.Errorf("Extra spiegelt die Umgebungsvariable nicht: %q", got)
	}
	if got := cfg.Extra["QUEUE_MAP_URL"]; got != "http://override.invalid/map.json" {
		t.Errorf("neuer Schluessel aus der Umgebung fehlt: %q", got)
	}
}

func TestLoadDefaultsPort(t *testing.T) {
	path := writeConfig(t, `{"JOB_PATH":"C:/Dashboard","MODEL_PATH":"C:/Model"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "4000" {
		t.Errorf("Port = %q, erwartet den Standardwert 4000", cfg.Port)
	}
}

func TestLoadAcceptsNumbersAndBooleans(t *testing.T) {
	path := writeConfig(t, `{"JOB_PATH":"C:/D","MODEL_PATH":"C:/M","LOCAL_SERVER_PORT":4000,"DEBUG":true}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "4000" {
		t.Errorf("Port = %q, erwartet %q", cfg.Port, "4000")
	}
	if cfg.Extra["DEBUG"] != "true" {
		t.Errorf("DEBUG = %q, erwartet %q", cfg.Extra["DEBUG"], "true")
	}
}

func TestLoadRejectsMissingRequiredKeys(t *testing.T) {
	path := writeConfig(t, `{"MODEL_PATH":"C:/Model"}`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load ohne JOB_PATH muss fehlschlagen")
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	path := writeConfig(t, `{"JOB_PATH":`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load mit kaputtem JSON muss fehlschlagen")
	}
}

func TestLoadWorksWithoutFileWhenEnvironmentIsComplete(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gibtesnicht.json")

	t.Setenv(EnvPrefix+KeyJobPath, "D:/Jobs")
	t.Setenv(EnvPrefix+KeyModelPath, "D:/Model")

	cfg, err := Load(missing)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JobPath != "D:/Jobs" || cfg.Port != "4000" {
		t.Errorf("unerwartete Konfiguration: %+v", cfg)
	}
}

func TestSetPortUpdatesExtra(t *testing.T) {
	path := writeConfig(t, `{"JOB_PATH":"C:/D","MODEL_PATH":"C:/M","LOCAL_SERVER_PORT":"4000"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cfg.SetPort("4001")

	if cfg.Port != "4001" || cfg.Extra[KeyPort] != "4001" {
		t.Errorf("SetPort hat nicht beide Stellen aktualisiert: %q / %q", cfg.Port, cfg.Extra[KeyPort])
	}
}
