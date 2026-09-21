// Package config liest die Laufzeitkonfiguration des Jobs-Backends aus einer
// JSON-Datei und erlaubt, jeden Schluessel ueber eine Umgebungsvariable zu
// ueberschreiben. Es ersetzt die Datei customisation/jobs.config.js des
// Node-Originals, die als CommonJS-Modul eingebunden wurde.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

const (
	// KeyJobPath ist das Verzeichnis, in dem Jobs und Logdateien liegen.
	KeyJobPath = "JOB_PATH"
	// KeyModelPath ist das Verzeichnis mit den Modell-JSON-Dateien.
	KeyModelPath = "MODEL_PATH"
	// KeyPort ist der Port, auf dem der Server lauscht.
	KeyPort = "LOCAL_SERVER_PORT"

	// EnvPrefix wird jedem Schluessel vorangestellt, um ihn ueber eine
	// Umgebungsvariable zu ueberschreiben, z.B. SOA_JOBS_JOB_PATH.
	EnvPrefix = "SOA_JOBS_"

	// DefaultPort ist der Port, der ohne eigene Angabe verwendet wird.
	DefaultPort = "4000"
)

// Config haelt die aufgeloeste Konfiguration. Extra enthaelt saemtliche
// Schluessel - auch die drei typisierten - damit /checkalive und
// /config/{name} beliebige einsatzspezifische Werte ausliefern koennen.
type Config struct {
	JobPath   string
	ModelPath string
	Port      string
	Extra     map[string]string
}

// Load liest die Konfigurationsdatei und wendet die Umgebungsvariablen an.
// Eine fehlende Datei ist zulaessig, solange die Pflichtwerte aus der
// Umgebung kommen.
func Load(path string) (*Config, error) {
	values := map[string]string{}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		values, err = decode(data)
		if err != nil {
			return nil, fmt.Errorf("Konfiguration %s: %w", path, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		// Datei ist optional.
	default:
		return nil, fmt.Errorf("Konfiguration %s: %w", path, err)
	}

	applyEnvironment(values, os.Environ())

	if values[KeyPort] == "" {
		values[KeyPort] = DefaultPort
	}

	for _, key := range []string{KeyJobPath, KeyModelPath} {
		if values[key] == "" {
			return nil, fmt.Errorf("Konfiguration %s: Pflichtschluessel %s fehlt", path, key)
		}
	}

	return &Config{
		JobPath:   values[KeyJobPath],
		ModelPath: values[KeyModelPath],
		Port:      values[KeyPort],
		Extra:     values,
	}, nil
}

// SetPort ueberschreibt den Port und haelt Extra konsistent, damit
// /checkalive und /config/LOCAL_SERVER_PORT den wirksamen Wert melden.
func (c *Config) SetPort(port string) {
	c.Port = port
	c.Extra[KeyPort] = port
}

// decode wandelt das JSON-Objekt in eine flache Zeichenkettenabbildung. Zahlen
// und Wahrheitswerte werden akzeptiert, weil die JS-Konfiguration sie
// gelegentlich unquotiert enthielt.
func decode(data []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var parsed map[string]any
	if err := dec.Decode(&parsed); err != nil {
		return nil, err
	}

	values := make(map[string]string, len(parsed))
	for key, value := range parsed {
		switch typed := value.(type) {
		case string:
			values[key] = typed
		case json.Number:
			values[key] = typed.String()
		case bool:
			values[key] = strconv.FormatBool(typed)
		default:
			return nil, fmt.Errorf("Schluessel %s: nur Zeichenketten, Zahlen und Wahrheitswerte sind erlaubt", key)
		}
	}
	return values, nil
}

// applyEnvironment uebernimmt jede Variable mit dem Praefix EnvPrefix.
func applyEnvironment(values map[string]string, environment []string) {
	for _, entry := range environment {
		name, value, found := strings.Cut(entry, "=")
		if !found || !strings.HasPrefix(name, EnvPrefix) {
			continue
		}
		values[strings.TrimPrefix(name, EnvPrefix)] = value
	}
}
