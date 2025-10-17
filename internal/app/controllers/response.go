package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
)

var ErrInvalidParam = errors.New("invalid param")

type errorResponse struct { Error string `json:"error"` }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, errorResponse{Error: err.Error()})
}
