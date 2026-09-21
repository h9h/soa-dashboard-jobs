package httpapi

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

// okHandler antwortet schlicht mit 200 und einem kurzen Text.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "ok")
})

// silentHandler beruehrt den ResponseWriter ueberhaupt nicht.
var silentHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

func TestWithTimingSetsResponseTimeHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)

	withTiming(okHandler).ServeHTTP(recorder, request)

	value := recorder.Header().Get("X-Response-Time")
	if value == "" {
		t.Fatal("X-Response-Time fehlt")
	}
	if _, err := strconv.Atoi(value); err != nil {
		t.Errorf("X-Response-Time = %q, erwartet eine Zahl in Millisekunden", value)
	}
}

func TestWithCORSReflectsOrigin(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	withCORS(okHandler).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, erwartet Origin", got)
	}
}

func TestWithCORSWithoutOriginSetsNoHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)

	withCORS(okHandler).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, erwartet leer", got)
	}
}

func TestWithCORSAnswersPreflight(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/job/save", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "content-type")

	reached := false
	guard := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	withCORS(guard).ServeHTTP(recorder, request)

	if reached {
		t.Error("Preflight darf den Handler dahinter nicht erreichen")
	}
	if recorder.Code != http.StatusNoContent {
		t.Errorf("Status = %d, erwartet 204", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Access-Control-Allow-Methods = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "content-type" {
		t.Errorf("Access-Control-Allow-Headers = %q", got)
	}
}

func TestWithBodyLimitRejectsOversizedBodies(t *testing.T) {
	var readErr error
	reader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})

	oversized := strings.NewReader(strings.Repeat("x", maxBodyBytes+1))
	request := httptest.NewRequest(http.MethodPost, "/job/save", oversized)

	withBodyLimit(reader).ServeHTTP(httptest.NewRecorder(), request)

	if readErr == nil {
		t.Fatal("ein Rumpf ueber der Grenze muss beim Lesen einen Fehler liefern")
	}
}

func TestWithBodyLimitAllowsNormalBodies(t *testing.T) {
	var body []byte
	reader := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
	})

	request := httptest.NewRequest(http.MethodPost, "/job/save", strings.NewReader("klein"))

	withBodyLimit(reader).ServeHTTP(httptest.NewRecorder(), request)

	if string(body) != "klein" {
		t.Errorf("Rumpf = %q", body)
	}
}

func TestWithLoggingSkipsNoisyRoutes(t *testing.T) {
	for _, target := range []string{"/checkalive", "/log"} {
		if !skipLogging(target) {
			t.Errorf("skipLogging(%q) = false, erwartet true", target)
		}
	}
	for _, target := range []string{"/jobs", "/job/eins.job.json", "/checkalive?x=1"} {
		if skipLogging(target) {
			t.Errorf("skipLogging(%q) = true, erwartet false", target)
		}
	}
}

func TestWithLoggingEmitsLogLine(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)

	withLogging(withTiming(okHandler)).ServeHTTP(recorder, request)

	output := buf.String()
	if !strings.Contains(output, "GET") {
		t.Errorf("Protokoll enthaelt GET nicht: %q", output)
	}
	if !strings.Contains(output, "/jobs") {
		t.Errorf("Protokoll enthaelt /jobs nicht: %q", output)
	}
	fields := strings.Fields(output)
	if len(fields) < 3 || fields[len(fields)-1] != "ms" || fields[len(fields)-3] != "-" {
		t.Errorf("Protokoll hat falsches Format: %q", output)
	}
	if _, err := strconv.Atoi(fields[len(fields)-2]); err != nil {
		t.Errorf("Protokoll enthaelt keine Millisekunden: %q", output)
	}
}

func TestWithLoggingSkipsCheckalive(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/checkalive", nil)

	withLogging(withTiming(okHandler)).ServeHTTP(recorder, request)

	if buf.String() != "" {
		t.Errorf("Protokoll sollte leer sein fuer /checkalive, aber: %q", buf.String())
	}
}

func TestWithLoggingLogsCheckaliveWithQuery(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/checkalive?x=1", nil)

	withLogging(withTiming(okHandler)).ServeHTTP(recorder, request)

	output := buf.String()
	if output == "" {
		t.Error("Protokoll sollte Eintrag fuer /checkalive?x=1 enthalten")
	}
	if !strings.Contains(output, "/checkalive?x=1") {
		t.Errorf("Protokoll enthaelt /checkalive?x=1 nicht: %q", output)
	}
}

func TestWithLoggingWithoutTimingLayer(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/job/save", nil)

	withLogging(okHandler).ServeHTTP(recorder, request)

	output := buf.String()
	if output == "" {
		t.Error("Protokoll sollte Eintrag fuer /job/save enthalten auch ohne Timing-Layer")
	}
	if !strings.Contains(output, "POST") {
		t.Errorf("Protokoll enthaelt POST nicht: %q", output)
	}
}

func TestWithTimingSetsResponseTimeHeaderExplicitWriteHeader(t *testing.T) {
	emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)

	withTiming(emptyHandler).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200", recorder.Code)
	}
	value := recorder.Header().Get("X-Response-Time")
	if value == "" {
		t.Fatal("X-Response-Time fehlt bei leerem Rumpf")
	}
	if _, err := strconv.Atoi(value); err != nil {
		t.Errorf("X-Response-Time = %q, erwartet eine Zahl in Millisekunden", value)
	}
}

func TestWithTimingSetsResponseTimeHeaderSilentHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jobs", nil)

	withTiming(silentHandler).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("Status = %d, erwartet 200", recorder.Code)
	}
	value := recorder.Header().Get("X-Response-Time")
	if value == "" {
		t.Fatal("X-Response-Time fehlt bei Handler der nichts beruehrt")
	}
	if _, err := strconv.Atoi(value); err != nil {
		t.Errorf("X-Response-Time = %q, erwartet eine Zahl in Millisekunden", value)
	}
}

func TestWithCORSPassesThroughNonPreflightOptions(t *testing.T) {
	reached := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		_, _ = io.WriteString(w, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/job/save", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	withCORS(handler).ServeHTTP(recorder, request)

	if !reached {
		t.Error("OPTIONS ohne Access-Control-Request-Method sollte den Handler erreichen")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
}
