package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, sts.ErrAccessDenied) {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeError(w, http.StatusBadGateway, "session operation failed")
}
