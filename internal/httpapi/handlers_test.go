package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"soa-dashboard-jobs/internal/config"
	"soa-dashboard-jobs/internal/jobstore"
)

// newTestServer baut einen Server auf temporaeren Verzeichnissen.
func newTestServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()

	root := t.TempDir()
	jobRoot := filepath.Join(root, "jobs")
	modelRoot := filepath.Join(root, "model")
	for _, dir := range []string{jobRoot, modelRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}

	cfg := &config.Config{
		JobPath:   jobRoot,
		ModelPath: modelRoot,
		Port:      "4000",
		Extra: map[string]string{
			config.KeyJobPath:   jobRoot,
			config.KeyModelPath: modelRoot,
			config.KeyPort:      "4000",
			"QUEUE_MAP_URL":     "http://example.invalid/map.json",
			"LEER":              "",
		},
	}

	server := NewServer(cfg, jobstore.New(jobRoot, modelRoot), "1.2.3")
	return server.Handler(), jobRoot, modelRoot
}

// do fuehrt eine Anfrage aus und liefert Status und Rumpf.
func do(t *testing.T, handler http.Handler, method, target string, body string) (int, []byte) {
	t.Helper()

	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, target, nil)
	} else {
		request = httptest.NewRequest(method, target, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.Bytes()
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

func TestCheckAliveShape(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, body := do(t, handler, http.MethodGet, "/checkalive", "")
	if status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}

	var parsed struct {
		Result       bool              `json:"result"`
		Env          map[string]string `json:"env"`
		ProcessStart string            `json:"process-start"`
		UptimeInMS   int64             `json:"uptime-in-ms"`
		Version      string            `json:"version"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v (%s)", err, body)
	}

	if !parsed.Result {
		t.Error("result = false")
	}
	if parsed.Version != "1.2.3" {
		t.Errorf("version = %q", parsed.Version)
	}
	if parsed.ProcessStart != "a few seconds ago" {
		t.Errorf("process-start = %q", parsed.ProcessStart)
	}
	if parsed.UptimeInMS < 0 {
		t.Errorf("uptime-in-ms = %d", parsed.UptimeInMS)
	}
	if parsed.Env["QUEUE_MAP_URL"] != "http://example.invalid/map.json" {
		t.Errorf("env enthaelt die Konfiguration nicht: %v", parsed.Env)
	}
}

func TestListJobs(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)
	writeFile(t, jobRoot, "eins.job.json", "{}")
	writeFile(t, jobRoot, "egal.txt", "x")

	_, body := do(t, handler, http.MethodGet, "/jobs", "")

	if string(body) != `{"jobs":["eins.job.json"]}` {
		t.Errorf("Antwort = %s", body)
	}
}

func TestListJobsOnEmptyDirectoryReturnsEmptyArray(t *testing.T) {
	handler, _, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodGet, "/jobs", "")

	if string(body) != `{"jobs":[]}` {
		t.Errorf("Antwort = %s, erwartet ein leeres Array statt null", body)
	}
}

func TestGetJobReturnsRawContent(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)
	writeFile(t, jobRoot, "eins.job.json", `{"a":1,"xml":"<b>&</b>"}`)

	_, body := do(t, handler, http.MethodGet, "/job/eins.job.json", "")

	want := `{"status":"ok","job":"{\"a\":1,\"xml\":\"<b>&</b>\"}"}`
	if string(body) != want {
		t.Errorf("Antwort =\n  %s\nerwartet\n  %s", body, want)
	}
}

func TestGetJobMissingFileReportsStatusOnly(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, body := do(t, handler, http.MethodGet, "/job/gibtesnicht.job.json", "")
	if status != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200 auch im Fehlerfall", status)
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v", err)
	}
	if _, present := parsed["job"]; present {
		t.Errorf("job darf im Fehlerfall nicht gesetzt sein: %s", body)
	}
	if parsed["status"] == "ok" {
		t.Errorf("status = ok, erwartet eine Fehlermeldung: %s", body)
	}
}

func TestGetJobEmptyFileKeepsJobField(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)
	writeFile(t, jobRoot, "leer.job.json", "")

	_, body := do(t, handler, http.MethodGet, "/job/leer.job.json", "")

	if string(body) != `{"status":"ok","job":""}` {
		t.Errorf("Antwort = %s", body)
	}
}

func TestGetModelAppendsExtension(t *testing.T) {
	handler, _, modelRoot := newTestServer(t)
	writeFile(t, modelRoot, "partner.json", `{"m":1}`)

	_, body := do(t, handler, http.MethodGet, "/model/partner", "")

	want := `{"status":"ok","model":"{\"m\":1}"}`
	if string(body) != want {
		t.Errorf("Antwort = %s, erwartet %s", body, want)
	}
}

func TestGetConfig(t *testing.T) {
	handler, _, _ := newTestServer(t)

	cases := []struct {
		name string
		want string
	}{
		{"QUEUE_MAP_URL", `{"status":"ok","config":"http://example.invalid/map.json"}`},
		{"LEER", `{"status":"nok","config":""}`},
		{"GIBTESNICHT", `{"status":"nok"}`},
	}

	for _, tc := range cases {
		_, body := do(t, handler, http.MethodGet, "/config/"+tc.name, "")
		if string(body) != tc.want {
			t.Errorf("/config/%s = %s, erwartet %s", tc.name, body, tc.want)
		}
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, _ := do(t, handler, http.MethodGet, "/gibtesnicht", "")
	if status != http.StatusNotFound {
		t.Errorf("Status = %d, erwartet 404", status)
	}
}
