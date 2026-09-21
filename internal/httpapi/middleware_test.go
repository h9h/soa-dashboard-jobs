package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// okHandler antwortet schlicht mit 200 und einem kurzen Text.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "ok")
})

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
