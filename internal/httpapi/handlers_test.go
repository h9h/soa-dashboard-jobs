package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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
	if _, err := time.Parse(time.RFC3339, parsed.ProcessStart); err != nil {
		t.Errorf("process-start = %q, erwartet ein RFC-3339-Zeitstempel: %v", parsed.ProcessStart, err)
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

// TestHandlerWiresAllMiddleware fuehrt eine Anfrage durch den echten, von
// Server.Handler() verdrahteten Handler und prueft in einem Zug alle vier
// beobachtbaren Effekte der Middlewarekette. Ein Mutationstest hat gezeigt,
// dass die uebrigen Tests in diesem Paket gruen bleiben, selbst wenn
// Handler() nur noch "return mux" macht oder withCORS herausgenommen wird -
// dieser Test soll genau das aufdecken.
func TestHandlerWiresAllMiddleware(t *testing.T) {
	handler, _, _ := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, withCORS scheint nicht verdrahtet zu sein", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, erwartet Origin", got)
	}

	responseTime := recorder.Header().Get("X-Response-Time")
	if responseTime == "" {
		t.Fatal("X-Response-Time fehlt, withTiming scheint nicht verdrahtet zu sein")
	}
	if _, err := strconv.Atoi(responseTime); err != nil {
		t.Errorf("X-Response-Time = %q, erwartet eine Zahl in Millisekunden", responseTime)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, erwartet application/json; charset=utf-8 - die Route scheint nicht erreicht worden zu sein", got)
	}
}

// TestHandlerAnswersPreflightWithoutReachingRoute prueft, dass ein Preflight
// durch den echten Handler() bereits von withCORS mit 204 beantwortet wird,
// bevor Logging, Timing oder eine Route ueberhaupt erreicht werden.
func TestHandlerAnswersPreflightWithoutReachingRoute(t *testing.T) {
	handler, _, _ := newTestServer(t)

	request := httptest.NewRequest(http.MethodOptions, "/jobs", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "GET")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("Status = %d, erwartet 204", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("Preflight-Antwort sollte leer sein, war aber %q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got == "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, der Preflight hat offenbar eine Route erreicht", got)
	}
}

// TestHandlerDoesNotLogPreflight treibt einen Preflight durch den echten,
// von Server.Handler() verdrahteten Handler und prueft, dass dabei nichts
// protokolliert wird. Anders als TestHandlerAnswersPreflightWithoutReachingRoute
// (Status und Header sind bei einem Preflight unabhaengig von der
// Middlewarereihenfolge gleich) unterscheidet dieser Test tatsaechlich
// zwischen der korrigierten und der urspruenglichen Reihenfolge: nur wenn
// withCORS ausserhalb von withLogging liegt, erreicht der Preflight die
// Logging-Middleware nicht.
func TestHandlerDoesNotLogPreflight(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	handler, _, _ := newTestServer(t)

	request := httptest.NewRequest(http.MethodOptions, "/job/save", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "POST")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("Status = %d, erwartet 204", recorder.Code)
	}
	if buf.String() != "" {
		t.Errorf("Protokoll sollte bei einem Preflight durch Handler() leer sein, war aber: %q", buf.String())
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, _ := do(t, handler, http.MethodGet, "/gibtesnicht", "")
	if status != http.StatusNotFound {
		t.Errorf("Status = %d, erwartet 404", status)
	}
}

func TestSaveJobWritesFile(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPost, "/job/save",
		`{"jobname":"lauf","chunk":"{\"a\":1}","append":false}`)

	if string(body) != `{"result":"ok"}` {
		t.Fatalf("Antwort = %s", body)
	}

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != `{"a":1}` {
		t.Errorf("Datei = %q", content)
	}
}

func TestSaveJobAppendsChunks(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)

	do(t, handler, http.MethodPost, "/job/save", `{"jobname":"lauf","chunk":"eins","append":false}`)
	do(t, handler, http.MethodPost, "/job/save", `{"jobname":"lauf","chunk":"zwei","append":true}`)

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "einszwei" {
		t.Errorf("Datei = %q", content)
	}
}

func TestSaveJobMissingChunkIsRejectedWithoutCreatingFile(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPost, "/job/save", `{"jobname":"lauf","append":false}`)

	if string(body) != `{"result":"missing chunk"}` {
		t.Errorf("Antwort = %s", body)
	}
	if _, err := os.Stat(filepath.Join(jobRoot, "lauf.job.json")); !os.IsNotExist(err) {
		t.Errorf("Datei haette nicht angelegt werden duerfen: Stat-Fehler = %v", err)
	}
}

func TestSaveJobMissingChunkDoesNotTouchExistingFile(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)
	writeFile(t, jobRoot, "lauf.job.json", "vorhandener inhalt")

	_, body := do(t, handler, http.MethodPost, "/job/save", `{"jobname":"lauf","append":false}`)

	if string(body) != `{"result":"missing chunk"}` {
		t.Errorf("Antwort = %s", body)
	}
	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "vorhandener inhalt" {
		t.Errorf("Datei wurde veraendert: %q", content)
	}
}

func TestSaveJobExplicitEmptyChunkIsHonoured(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPost, "/job/save", `{"jobname":"lauf","chunk":"","append":false}`)

	if string(body) != `{"result":"ok"}` {
		t.Fatalf("Antwort = %s", body)
	}
	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.job.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "" {
		t.Errorf("Datei = %q, erwartet eine leere, aber angelegte Datei", content)
	}
}

func TestSaveJobRejectsPathTraversal(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, body := do(t, handler, http.MethodPost, "/job/save",
		`{"jobname":"../ausbruch","chunk":"x","append":false}`)

	if status != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200", status)
	}
	if string(body) != `{"result":"invalid file"}` {
		t.Errorf("Antwort = %s", body)
	}
}

func TestLogAppendsWithoutDestinationAndKeepsOrder(t *testing.T) {
	handler, jobRoot, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPut, "/log",
		`{"destination":"lauf","timestamp":1758445200000,"zeta":"z","alpha":"<a>&</a>"}`)

	if string(body) != `{"result":"ok"}` {
		t.Fatalf("Antwort = %s", body)
	}

	content, err := os.ReadFile(filepath.Join(jobRoot, "lauf.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "{\"timestamp\":1758445200000,\"zeta\":\"z\",\"alpha\":\"<a>&</a>\"},\n"
	if string(content) != want {
		t.Errorf("Datei =\n  %q\nerwartet\n  %q", content, want)
	}
}

func TestLogRejectsPathTraversal(t *testing.T) {
	handler, _, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPut, "/log", `{"destination":"../ausbruch","a":1}`)

	if string(body) != `{"result":"invalid file"}` {
		t.Errorf("Antwort = %s", body)
	}
}

func TestLogRejectsBrokenJSON(t *testing.T) {
	handler, _, _ := newTestServer(t)

	status, body := do(t, handler, http.MethodPut, "/log", `{"destination":`)

	if status != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200", status)
	}
	if string(body) == `{"result":"ok"}` {
		t.Errorf("kaputtes JSON darf nicht als ok gemeldet werden: %s", body)
	}
}

func TestSaveJobRejectsBrokenJSON(t *testing.T) {
	handler, _, _ := newTestServer(t)

	_, body := do(t, handler, http.MethodPost, "/job/save", `{"jobname":`)

	if string(body) == `{"result":"ok"}` {
		t.Errorf("kaputtes JSON darf nicht als ok gemeldet werden: %s", body)
	}
}
