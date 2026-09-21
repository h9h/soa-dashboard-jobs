// Package jobstore kapselt saemtliche Dateizugriffe des Jobs-Backends.
// Es kennt kein HTTP.
package jobstore

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// JobExt ist die Endung, an der Jobdateien erkannt werden.
	JobExt   = ".job.json"
	modelExt = ".json"
	logExt   = ".log"

	filePermissions = 0o644
)

// ErrInvalidFile meldet einen Pfad ausserhalb des zulaessigen Verzeichnisses.
// Der Text entspricht der Antwort des Node-Originals.
var ErrInvalidFile = errors.New("invalid file")

// Store liest und schreibt unterhalb zweier fester Wurzelverzeichnisse.
type Store struct {
	JobRoot   string
	ModelRoot string
}

// New normalisiert die Wurzelverzeichnisse einmalig.
func New(jobRoot, modelRoot string) *Store {
	return &Store{
		JobRoot:   filepath.Clean(jobRoot),
		ModelRoot: filepath.Clean(modelRoot),
	}
}

// ListJobs liefert die Namen aller Jobdateien. Ein nicht lesbares Verzeichnis
// ist kein Fehler fuer den Aufrufer, sondern eine leere Liste mit
// Protokolleintrag - wie im Node-Original.
func (s *Store) ListJobs() []string {
	entries, err := os.ReadDir(s.JobRoot)
	if err != nil {
		log.Printf("Verzeichnis kann nicht gelesen werden: %v", err)
		return []string{}
	}

	jobs := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), JobExt) {
			jobs = append(jobs, entry.Name())
		}
	}
	return jobs
}

// GetJob liest eine Jobdatei. Der Name wird unveraendert verwendet; das
// Frontend haengt .job.json selbst an.
func (s *Store) GetJob(name string) (string, error) {
	return read(s.JobRoot, name)
}

// GetModel liest eine Modelldatei und ergaenzt .json, falls noetig.
func (s *Store) GetModel(name string) (string, error) {
	return read(s.ModelRoot, withExtension(name, modelExt))
}

// SaveJob schreibt chunk in eine Jobdatei. Bei appendMode wird angehaengt,
// sonst die Datei neu angelegt. chunk ist ein roher Ausschnitt und wird nicht
// als JSON interpretiert - das Frontend streamt Jobs in 64-KiB-Stuecken.
func (s *Store) SaveJob(name, chunk string, appendMode bool) error {
	path, err := resolve(s.JobRoot, withExtension(name, JobExt))
	if err != nil {
		return err
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	return writeFile(path, flags, []byte(chunk))
}

// AppendLog haengt payload gefolgt von ",\n" an eine Logdatei an.
func (s *Store) AppendLog(destination string, payload []byte) error {
	path, err := resolve(s.JobRoot, withExtension(destination, logExt))
	if err != nil {
		return err
	}

	line := make([]byte, 0, len(payload)+2)
	line = append(line, payload...)
	line = append(line, ',', '\n')

	return writeFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, line)
}

func read(root, name string) (string, error) {
	path, err := resolve(root, name)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func writeFile(path string, flags int, content []byte) error {
	file, err := os.OpenFile(path, flags, filePermissions)
	if err != nil {
		return err
	}

	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// withExtension haengt ext an, wenn der Name nicht bereits darauf endet.
func withExtension(name, ext string) string {
	if strings.HasSuffix(name, ext) {
		return name
	}
	return name + ext
}

// resolve bildet name auf einen Pfad direkt in root ab. Zulaessig ist nur das
// Wurzelverzeichnis selbst - keine Unterverzeichnisse, kein Ausbrechen ueber
// "..". Das entspricht checkStaysInDirectory des Node-Originals, das
// path.dirname(pfad) mit dem Wurzelverzeichnis vergleicht.
func resolve(root, name string) (string, error) {
	path := filepath.Join(root, name)
	if !samePath(filepath.Dir(path), root) {
		return "", ErrInvalidFile
	}
	return path, nil
}

// samePath vergleicht zwei bereits bereinigte Pfade. Unter Windows ist der
// Vergleich unabhaengig von der Gross- und Kleinschreibung, weil das
// Dateisystem sie ebenfalls ignoriert.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
