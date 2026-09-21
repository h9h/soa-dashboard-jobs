package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"soa-dashboard-jobs/internal/jobstore"
	"soa-dashboard-jobs/internal/jsonutil"
)

// handleCheckAlive meldet Laufzeit, Version und die wirksame Konfiguration.
func (s *Server) handleCheckAlive(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.start)

	writeJSON(w, checkAliveResponse{
		Result:       true,
		Env:          s.cfg.Extra,
		ProcessStart: s.start.Format(time.RFC3339),
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
		writeJSON(w, jobResponse{Status: errorText(err)})
		return
	}
	writeJSON(w, jobResponse{Status: "ok", Job: &content})
}

// handleGetModel liefert den Inhalt einer Modelldatei als Zeichenkette.
func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	content, err := s.store.GetModel(r.PathValue("name"))
	if err != nil {
		writeJSON(w, modelResponse{Status: errorText(err)})
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

// saveJobRequest ist der Rumpf von POST /job/save. chunk ist ein roher
// Textausschnitt, den das Frontend in 64-KiB-Stuecken sendet. Chunk ist ein
// Zeiger, damit ein fehlendes Feld ("kein chunk gesendet") von einer
// tatsaechlich leeren Zeichenkette ("chunk":"") unterschieden werden kann -
// nur Ersteres darf die Datei nicht anfassen.
type saveJobRequest struct {
	Jobname string  `json:"jobname"`
	Append  bool    `json:"append"`
	Chunk   *string `json:"chunk"`
}

// handleSaveJob schreibt einen Ausschnitt in eine Jobdatei. Fehlt chunk im
// Rumpf, wird die Datei bewusst nicht angefasst - das Node-Original warf in
// diesem Fall vor jedem Dateizugriff eine Ausnahme.
func (s *Server) handleSaveJob(w http.ResponseWriter, r *http.Request) {
	var request saveJobRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, resultResponse{Result: err.Error()})
		return
	}

	if request.Chunk == nil {
		writeJSON(w, resultResponse{Result: "missing chunk"})
		return
	}

	if err := s.store.SaveJob(request.Jobname, *request.Chunk, request.Append); err != nil {
		writeJSON(w, resultResponse{Result: errorText(err)})
		return
	}
	writeJSON(w, resultResponse{Result: "ok"})
}

// handleLog haengt den Rumpf ohne das Feld destination an eine Logdatei an.
func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, resultResponse{Result: err.Error()})
		return
	}

	var envelope struct {
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeJSON(w, resultResponse{Result: err.Error()})
		return
	}

	payload, err := jsonutil.StripKey(body, "destination")
	if err != nil {
		writeJSON(w, resultResponse{Result: err.Error()})
		return
	}

	if err := s.store.AppendLog(envelope.Destination, payload); err != nil {
		writeJSON(w, resultResponse{Result: errorText(err)})
		return
	}
	writeJSON(w, resultResponse{Result: "ok"})
}

// errorText liefert fuer abgelehnte Pfade genau die Meldung des
// Node-Originals und sonst den Text des Go-Fehlers.
func errorText(err error) string {
	if errors.Is(err, jobstore.ErrInvalidFile) {
		return jobstore.ErrInvalidFile.Error()
	}
	return err.Error()
}
