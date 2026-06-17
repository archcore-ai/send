package api

import (
	"errors"
	"net/http"

	"github.com/ivklgn/archcore-send/internal/store"
)

// errorBody is the server error shape (error-catalog): no remediation (client-side),
// no sensitive data.
type errorBody struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

// writeError writes a JSON error and records the coarse code for the request log.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	if sw, ok := w.(*statusWriter); ok {
		sw.errCode = code
	}
	writeJSON(w, status, errorBody{ErrorCode: code, Message: msg})
}

// mapStoreError maps a store sentinel error to its default HTTP status + code.
// Handlers that need a context-specific mapping (e.g. unknown part on download
// vs upload) handle those cases before falling through to this.
func mapStoreError(err error) (int, string, string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound, "SEND_NOT_FOUND", "send not found"
	case errors.Is(err, store.ErrNotFinalized):
		return http.StatusConflict, "SEND_NOT_FINALIZED", "send is not finalized"
	case errors.Is(err, store.ErrConflict):
		return http.StatusConflict, "SEND_FINALIZED", "send is no longer accepting uploads"
	case errors.Is(err, store.ErrAlreadyRedeemed):
		return http.StatusGone, "SEND_ALREADY_REDEEMED", "send has already been redeemed"
	case errors.Is(err, store.ErrExpired):
		return http.StatusGone, "SEND_EXPIRED", "send has expired"
	case errors.Is(err, store.ErrIncomplete):
		return http.StatusBadRequest, "INCOMPLETE", "not all declared parts were uploaded"
	case errors.Is(err, store.ErrUnknownPart):
		return http.StatusBadRequest, "BAD_REQUEST", "unknown part id"
	case errors.Is(err, store.ErrIntegrity):
		return http.StatusUnprocessableEntity, "INTEGRITY_FAILED", "ciphertext failed integrity check"
	case errors.Is(err, store.ErrPartTooLarge):
		return http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "part exceeds the size cap"
	case errors.Is(err, store.ErrInvalidGrant):
		return http.StatusForbidden, "INVALID_REDEEM", "missing, invalid, or expired redeem grant"
	default:
		return http.StatusInternalServerError, "STORAGE_ERROR", "internal error"
	}
}

func writeStoreError(w http.ResponseWriter, err error) {
	status, code, msg := mapStoreError(err)
	writeError(w, status, code, msg)
}
