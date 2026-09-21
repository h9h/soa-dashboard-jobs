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
	flag.Usage = func() { fmt.Fprint(os.Stderr, usageText()) }
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	if err := applyPortArgument(cfg, flag.Args()); err != nil {
		fail(err)
	}

	dirCreated, err := ensureDir(cfg.JobPath)
	if err != nil {
		log.Printf("Verzeichnis %s konnte nicht angelegt werden: %v", cfg.JobPath, err)
	}

	store := jobstore.New(cfg.JobPath, cfg.ModelPath)
	server := httpapi.NewServer(cfg, store, version)

	fmt.Print(helpText(cfg, dirCreated))

	if err := http.ListenAndServe(listenAddress(cfg.Port), server.Handler()); err != nil {
		fail(fmt.Errorf("Server auf %s beendet: %w", listenAddress(cfg.Port), err))
	}
}

// fail bricht den Start ab. Neben der Ursache wird die Kurzhilfe ausgegeben,
// weil jeder Abbruchgrund - fehlende Pflichtschluessel, ungueltiger oder
// belegter Port - ueber die in usageText beschriebenen Parameter behoben wird.
// Beides geht nach stderr, damit die Ausgabe eines aufrufenden Skripts auf
// stdout unberuehrt bleibt.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "Start abgebrochen: %v\n", err)
	fmt.Fprint(os.Stderr, usageText())
	os.Exit(1)
}

// listenAddress bindet bewusst nur an die Loopback-Adresse: der Dienst
// kennt keine Authentisierung und spiegelt jeden Origin zurueck, deshalb
// koennte sonst jede Webseite, die der Anwender besucht, das Dateisystem im
// konfigurierten JOB_PATH veraendern. Sowohl die SPA als auch ein
// vorgelagerter Webserver sprechen den Dienst vom selben Rechner aus an.
func listenAddress(port string) string {
	return net.JoinHostPort("127.0.0.1", port)
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

// usageText beschreibt Aufruf und Parameter. Es wird bei jedem Startabbruch
// und bei -h ausgegeben.
func usageText() string {
	return fmt.Sprintf(`
Aufruf:
  %[1]s [-config <Pfad>] [Port]

Parameter:
  %-17[2]s Verzeichnis fuer Jobs und Logdateien (Pflicht)
  %-17[3]s Verzeichnis mit den Modelldaten (Pflicht)
  %-17[4]s Port des Servers (Vorgabe %[5]s)

Drei Wege, sie zu setzen - der spaetere gewinnt:

  1. Konfigurationsdatei %[6]s, gesucht im Arbeitsverzeichnis,
     danach neben der ausfuehrbaren Datei. Anderer Pfad ueber -config:
       %[1]s -config C:\Dienste\%[6]s
     Inhalt:
       {
         "%[2]s": "C:/Dashboard",
         "%[3]s": "C:/DashboardModel",
         "%[4]s": "%[5]s"
       }

  2. Umgebungsvariablen mit dem Praefix %[7]s - sie ueberschreiben
     die Datei, jeder Schluessel ist so setzbar:
       set %[7]s%[2]s=C:\Dashboard
       set %[7]s%[3]s=C:\DashboardModel
       set %[7]s%[4]s=%[5]s

  3. Erstes Positionsargument - setzt nur den Port (1-65535):
       %[1]s 4001
`, executableName(), config.KeyJobPath, config.KeyModelPath, config.KeyPort,
		config.DefaultPort, configFileName, config.EnvPrefix)
}

// executableName liefert den Namen der laufenden Datei, damit die Beispiele
// im Hilfetext auch nach einem Umbenennen der EXE aufrufbar bleiben.
func executableName() string {
	executable, err := os.Executable()
	if err != nil {
		return "soa-dashboard-jobs.exe"
	}
	return filepath.Base(executable)
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
