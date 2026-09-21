package httpapi

import (
	"net/http"
	"time"
)

// handleCheckAlive meldet Laufzeit, Version und die wirksame Konfiguration.
func (s *Server) handleCheckAlive(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.start)

	writeJSON(w, checkAliveResponse{
		Result:       true,
		Env:          s.cfg.Extra,
		ProcessStart: relativeTime(uptime),
		UptimeInMS:   uptime.Milliseconds(),
		Version:      s.version,
	})
}

// handleListJobs liefert die Namen aller Jobdateien.
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, jobsResponse{Jobs: s.store.ListJobs()})
}

// handleGetJob liefert den Inhalt einer Jobdatei als Zeichenkette.
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	content, err := s.store.GetJob(r.PathValue("jobname"))
	if err != nil {
		writeJSON(w, jobResponse{Status: err.Error()})
		return
	}
	writeJSON(w, jobResponse{Status: "ok", Job: &content})
}

// handleGetModel liefert den Inhalt einer Modelldatei als Zeichenkette.
func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	content, err := s.store.GetModel(r.PathValue("name"))
	if err != nil {
		writeJSON(w, modelResponse{Status: err.Error()})
		return
	}
	writeJSON(w, modelResponse{Status: "ok", Model: &content})
}

// handleGetConfig liefert einen einzelnen Konfigurationswert. Ein leerer Wert
// gilt als nok, weil das Node-Original auf Wahrheitswert geprueft hat.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	value, present := s.cfg.Extra[r.PathValue("name")]
	if !present {
		writeJSON(w, configResponse{Status: "nok"})
		return
	}

	status := "ok"
	if value == "" {
		status = "nok"
	}
	writeJSON(w, configResponse{Status: status, Config: &value})
}

// handleSaveJob und handleLog folgen in Task 7.
func (s *Server) handleSaveJob(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, resultResponse{Result: "not implemented"})
}

func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, resultResponse{Result: "not implemented"})
}
