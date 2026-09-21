package httpapi

import (
	"log"
	"net/http"
	"strconv"
	"time"
)

// maxBodyBytes entspricht dem jsonLimit von 32mb, das koa-bodyparser im
// Node-Original gesetzt hat.
const maxBodyBytes = 32 << 20

// allowedMethods entspricht der Standardliste von @koa/cors.
const allowedMethods = "GET,HEAD,PUT,POST,DELETE,PATCH"

// withBodyLimit begrenzt die Groesse des Anfragerumpfs.
func withBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// withCORS bildet die Voreinstellungen von @koa/cors nach: der Origin der
// Anfrage wird zurueckgespiegelt, Preflight-Anfragen werden mit 204
// beantwortet.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
				w.Header().Set("Access-Control-Allow-Headers", requested)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// timedWriter setzt X-Response-Time, sobald der Statuscode feststeht. Koa
// konnte den Header nachtraeglich setzen, weil es den Rumpf zwischenspeichert;
// in Go muss das vor dem ersten Schreiben passieren.
type timedWriter struct {
	http.ResponseWriter
	start       time.Time
	wroteHeader bool
}

func (w *timedWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	elapsed := time.Since(w.start).Milliseconds()
	w.Header().Set("X-Response-Time", strconv.FormatInt(elapsed, 10))
	w.ResponseWriter.WriteHeader(status)
}

func (w *timedWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// withTiming misst die Bearbeitungsdauer und meldet sie im Header.
func withTiming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &timedWriter{ResponseWriter: w, start: time.Now()}
		next.ServeHTTP(writer, r)

		// Antworten ohne Rumpf haben den Header noch nicht gesetzt.
		if !writer.wroteHeader {
			writer.WriteHeader(http.StatusOK)
		}
	})
}

// skipLogging unterdrueckt die beiden Routen, die das Frontend im Sekundentakt
// aufruft - genau wie das Node-Original.
func skipLogging(target string) bool {
	return target == "/checkalive" || target == "/log"
}

// withLogging protokolliert jede Anfrage mit der Dauer aus withTiming.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		target := r.URL.RequestURI()
		if skipLogging(target) {
			return
		}
		log.Printf("%s %s - %s ms", r.Method, target, w.Header().Get("X-Response-Time"))
	})
}
