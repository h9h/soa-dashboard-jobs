package jobstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// newTestStore legt Job- und Modellverzeichnis im Temp-Bereich an.
func newTestStore(t *testing.T) (*Store, string, string) {
	t.Helper()

	root := t.TempDir()
	jobRoot := filepath.Join(root, "jobs")
	modelRoot := filepath.Join(root, "model")

	for _, dir := range []string{jobRoot, modelRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Verzeichnis %s: %v", dir, err)
		}
	}
	return New(jobRoot, modelRoot), jobRoot, modelRoot
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("Datei %s: %v", name, err)
	}
}

func TestListJobsReturnsOnlyJobFiles(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	write(t, jobRoot, "eins.job.json", "{}")
	write(t, jobRoot, "zwei.job.json", "{}")
	write(t, jobRoot, "notizen.txt", "egal")
	write(t, jobRoot, "lauf.log", "egal")

	jobs := store.ListJobs()

	if len(jobs) != 2 {
		t.Fatalf("ListJobs = %v, erwartet zwei Eintraege", jobs)
	}
	for _, name := range jobs {
		if name != "eins.job.json" && name != "zwei.job.json" {
			t.Errorf("unerwarteter Eintrag %q", name)
		}
	}
}

func TestListJobsOnMissingDirectoryReturnsEmptySlice(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "gibtesnicht"), t.TempDir())

	jobs := store.ListJobs()

	if jobs == nil {
		t.Fatal("ListJobs darf nil nie zurueckgeben, sonst wird daraus JSON null statt []")
	}
	if len(jobs) != 0 {
		t.Errorf("ListJobs = %v, erwartet leer", jobs)
	}
}

func TestGetJobReturnsContentVerbatim(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)
	write(t, jobRoot, "eins.job.json", `{"a":1}`)

	content, err := store.GetJob("eins.job.json")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if content != `{"a":1}` {
		t.Errorf("GetJob = %q", content)
	}
}

func TestGetJobDoesNotAppendExtension(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)
	write(t, jobRoot, "eins.job.json", `{"a":1}`)

	// Das Node-Original haengt hier nichts an; das erledigt das Frontend.
	if _, err := store.GetJob("eins"); err == nil {
		t.Fatal("GetJob(\"eins\") muss fehlschlagen, die Datei heisst eins.job.json")
	}
}

func TestGetModelAppendsJSONExtension(t *testing.T) {
	store, _, modelRoot := newTestStore(t)
	write(t, modelRoot, "partner.json", `{"m":true}`)

	content, err := store.GetModel("partner")
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if content != `{"m":true}` {
		t.Errorf("GetModel = %q", content)
	}

	if _, err := store.GetModel("partner.json"); err != nil {
		t.Errorf("GetModel mit vorhandener Endung: %v", err)
	}
}

func TestSaveJobAppendsJobExtensionAndTruncates(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	if err := store.SaveJob("lauf", "erster", false); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	if err := store.SaveJob("lauf.job.json", "zweiter", false); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "zweiter" {
		t.Errorf("Datei = %q, erwartet %q - append=false muss abschneiden", content, "zweiter")
	}
}

func TestSaveJobAppends(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	if err := store.SaveJob("lauf", "eins", false); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}
	for _, chunk := range []string{"zwei", "drei"} {
		if err := store.SaveJob("lauf", chunk, true); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
	}

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "einszweidrei" {
		t.Errorf("Datei = %q", content)
	}
}

func TestAppendLogWritesPayloadWithCommaAndNewline(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	if err := store.AppendLog("lauf", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	if err := store.AppendLog("lauf.log", []byte(`{"a":2}`)); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "{\"a\":1},\n{\"a\":2},\n"
	if string(content) != want {
		t.Errorf("Datei = %q, erwartet %q", content, want)
	}
}

func TestPathsOutsideTheRootAreRejected(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	if err := os.MkdirAll(filepath.Join(jobRoot, "unter"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	names := []string{
		"../ausbruch",
		"../../ausbruch",
		"unter/tiefer",
		filepath.Join(t.TempDir(), "absolut"),
		"C:foo",
		"C:foo.job.json",
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := store.SaveJob(name, "inhalt", false); !errors.Is(err, ErrInvalidFile) {
				t.Errorf("SaveJob(%q) = %v, erwartet ErrInvalidFile", name, err)
			}
			if err := store.AppendLog(name, []byte("{}")); !errors.Is(err, ErrInvalidFile) {
				t.Errorf("AppendLog(%q) = %v, erwartet ErrInvalidFile", name, err)
			}
			if _, err := store.GetJob(name); !errors.Is(err, ErrInvalidFile) {
				t.Errorf("GetJob(%q) = %v, erwartet ErrInvalidFile", name, err)
			}
			if _, err := store.GetModel(name); !errors.Is(err, ErrInvalidFile) {
				t.Errorf("GetModel(%q) = %v, erwartet ErrInvalidFile", name, err)
			}
		})
	}
}

func TestRejectedPathsAreNotWritten(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	_ = store.SaveJob("../ausbruch", "inhalt", false)

	parent := filepath.Dir(jobRoot)
	if _, err := os.Stat(filepath.Join(parent, "ausbruch.job.json")); err == nil {
		t.Fatal("abgelehnter Pfad wurde trotzdem geschrieben")
	}
}

func TestNTFSAlternateDataStreamNamesAreRejected(t *testing.T) {
	store, jobRoot, _ := newTestStore(t)

	// NTFS-Doppelpunkte: SaveJob mit verschiedenen Varianten
	_ = store.SaveJob("C:foo", "inhalt", false)
	if _, err := os.Stat(filepath.Join(jobRoot, "C")); err == nil {
		t.Fatal("Datei C sollte nicht durch SaveJob(\"C:foo\", ...) entstanden sein")
	}

	// AppendLog mit Doppelpunkt und Endung
	_ = store.AppendLog("D:bar.log", []byte("{}"))
	if _, err := os.Stat(filepath.Join(jobRoot, "D")); err == nil {
		t.Fatal("Datei D sollte nicht durch AppendLog(\"D:bar.log\", ...) entstanden sein")
	}

	// GetJob darf nicht lesend zugreifen
	_, _ = store.GetJob("E:baz.job.json")
	if _, err := os.Stat(filepath.Join(jobRoot, "E")); err == nil {
		t.Fatal("Datei E sollte nicht durch GetJob(\"E:baz.job.json\") entstanden sein")
	}
}
