package legalotp

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type HTTPService struct {
	access *MatterAccess
	log    *slog.Logger
}

func NewHTTPService(access *MatterAccess, logger *slog.Logger) http.Handler {
	s := &HTTPService{access: access, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code/request", s.requestCode)
	mux.HandleFunc("POST /login/code/verify", s.verifyCode)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return requestLog(logger, mux)
}

func (s *HTTPService) requestCode(w http.ResponseWriter, r *http.Request) {
	var input MatterLogin
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return
	}
	result, err := s.access.RequestCode(r.Context(), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"matter_id": input.MatterID, "message_id": result.MessageID, "state": "code_sent",
	})
}

func (s *HTTPService) verifyCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		MatterID string `json:"matter_id"`
		Code     string `json:"code"`
	}
	if err := decodeJSON(r, &input); err != nil || input.MatterID == "" || input.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "matter_id and code are required"})
		return
	}
	decision, err := s.access.Verify(r.Context(), input.MatterID, input.Code)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *HTTPService) writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	message := err.Error()
	if apiErr, ok := IsAPIError(err); ok {
		status = apiErr.StatusCode
		if status < 400 || status >= 500 {
			status = http.StatusBadGateway
			message = "code service request failed"
		}
	}
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}
