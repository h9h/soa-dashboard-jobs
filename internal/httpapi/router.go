// Package httpapi stellt die REST-Schnittstelle des Jobs-Backends bereit.
// Die Antworten sind byteweise mit dem Koa-Original vertraeglich: jede Route
// antwortet mit 200, Fehler stehen im Rumpf.
package httpapi

import (
	"log"
	"net/http"
	"time"

	"soa-dashboard-jobs/internal/config"
	"soa-dashboard-jobs/internal/jobstore"
	"soa-dashboard-jobs/internal/jsonutil"
)

// Server buendelt die Abhaengigkeiten der Handler.
type Server struct {
	cfg     *config.Config
	store   *jobstore.Store
	version string
	start   time.Time
}

// NewServer merkt sich den Startzeitpunkt fuer /checkalive.
func NewServer(cfg *config.Config, store *jobstore.Store, version string) *Server {
	return &Server{
		cfg:     cfg,
		store:   store,
		version: version,
		start:   time.Now(),
	}
}

// Handler liefert den fertig verdrahteten Handler samt Middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /checkalive", s.handleCheckAlive)
	mux.HandleFunc("GET /jobs", s.handleListJobs)
	mux.HandleFunc("GET /job/{jobname}", s.handleGetJob)
	mux.HandleFunc("POST /job/save", s.handleSaveJob)
	mux.HandleFunc("GET /model/{name}", s.handleGetModel)
	mux.HandleFunc("GET /config/{name}", s.handleGetConfig)
	mux.HandleFunc("PUT /log", s.handleLog)

	return withBodyLimit(withCORS(withLogging(withTiming(mux))))
}

// Die Antworttypen sind Strukturen statt Abbildungen, damit die Reihenfolge
// der Felder der des Node-Originals entspricht. Zeiger mit omitempty bilden
// nach, dass JSON.stringify undefined weglaesst, "" aber ausgibt.

type resultResponse struct {
	Result string `json:"result"`
}

type jobsResponse struct {
	Jobs []string `json:"jobs"`
}

type jobResponse struct {
	Status string  `json:"status"`
	Job    *string `json:"job,omitempty"`
}

type modelResponse struct {
	Status string  `json:"status"`
	Model  *string `json:"model,omitempty"`
}

type configResponse struct {
	Status string  `json:"status"`
	Config *string `json:"config,omitempty"`
}

type checkAliveResponse struct {
	Result       bool              `json:"result"`
	Env          map[string]string `json:"env"`
	ProcessStart string            `json:"process-start"`
	UptimeInMS   int64             `json:"uptime-in-ms"`
	Version      string            `json:"version"`
}

// writeJSON schreibt die Antwort ohne HTML-Escaping und immer mit Status 200.
func writeJSON(w http.ResponseWriter, payload any) {
	body, err := jsonutil.Marshal(payload)
	if err != nil {
		log.Printf("Antwort konnte nicht kodiert werden: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		log.Printf("Antwort konnte nicht gesendet werden: %v", err)
	}
}
