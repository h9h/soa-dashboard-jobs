// Kommando soa-dashboard-jobs ist das Housekeeping-Backend des
// ESB/SOA-Dashboards. Es liest und schreibt Jobdefinitionen, Logdateien und
// Modelldaten in lokalen Verzeichnissen.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"soa-dashboard-jobs/internal/config"
	"soa-dashboard-jobs/internal/httpapi"
	"soa-dashboard-jobs/internal/jobstore"
)

// version wird beim Bauen gesetzt: -ldflags "-X main.version=1.2.3".
var version = "dev"

const configFileName = "jobs.config.json"

func main() {
	configPath := flag.String("config", defaultConfigPath(), "Pfad zur Konfigurationsdatei")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := applyPortArgument(cfg, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dirCreated, err := ensureDir(cfg.JobPath)
	if err != nil {
		log.Printf("Verzeichnis %s konnte nicht angelegt werden: %v", cfg.JobPath, err)
	}

	store := jobstore.New(cfg.JobPath, cfg.ModelPath)
	server := httpapi.NewServer(cfg, store, version)

	fmt.Print(helpText(cfg, dirCreated))

	address := net.JoinHostPort("", cfg.Port)
	if err := http.ListenAndServe(address, server.Handler()); err != nil {
		log.Fatalf("Server beendet: %v", err)
	}
}

// applyPortArgument uebernimmt den Port aus dem ersten Positionsargument.
// Das Node-Original hat process.argv[2] genauso ausgewertet, jedoch ohne
// Bereichspruefung. Diese Abweichung ist bewusst: sie verhindert, dass der
// Server unbemerkt auf einem von der API abweichenden Port lauscht (Port 0
// laesst das Betriebssystem einen zufaelligen Port waehlen).
func applyPortArgument(cfg *config.Config, args []string) error {
	if len(args) == 0 || args[0] == "" {
		return nil
	}
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("ungueltiger Port %q", args[0])
	}

	cfg.SetPort(args[0])
	return nil
}

// ensureDir legt das Jobverzeichnis an, falls es fehlt, und meldet, ob es neu
// entstanden ist. Anders als das Node-Original werden fehlende
// Elternverzeichnisse mit angelegt.
func ensureDir(dir string) (bool, error) {
	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s ist kein Verzeichnis", dir)
		}
		return false, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

// defaultConfigPath sucht die Konfiguration erst im Arbeitsverzeichnis, dann
// neben der ausfuehrbaren Datei.
func defaultConfigPath() string {
	if _, err := os.Stat(configFileName); err == nil {
		return configFileName
	}

	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), configFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return configFileName
}

// helpText ist das Startbanner, nachgebildet nach getHelpText des
// Node-Originals.
func helpText(cfg *config.Config, dirCreated bool) string {
	hinweis := ""
	if dirCreated {
		hinweis = " (neu angelegt)"
	}

	return fmt.Sprintf(`
ESB-Dashboard File Backend (Go, Version %s)
------------------------------------
Listening to http://localhost:%s

Local dir for jobs:
  %s%s

Local dir for model-data:
  %s

Routes:
  GET  /jobs
  GET  /job/:jobname
  POST /job/save

  GET  /model/:modelname
  GET  /config/:name

  PUT  /log
  GET  /checkalive
`, version, cfg.Port, cfg.JobPath, hinweis, cfg.ModelPath)
}
