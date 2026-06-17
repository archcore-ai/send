package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ivklgn/archcore-send/internal/store"
)

// maxCreateBody bounds the create request JSON (metadata only, never ciphertext).
const maxCreateBody = 1 << 20 // 1 MiB

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type createReq struct {
	Version    string `json:"version"`
	OneTime    *bool  `json:"one_time"`
	TTLSeconds int64  `json:"ttl_seconds"`
	Parts      []struct {
		PartID        string `json:"part_id"`
		EncryptedSize int64  `json:"encrypted_size"`
		SHA256        string `json:"sha256"`
	} `json:"parts"`
}

type partBrief struct {
	PartID        string `json:"part_id"`
	EncryptedSize int64  `json:"encrypted_size"`
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if !s.allow(r, "create") {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
		return
	}
	var req createReq
	if err := json.NewDecoder(io.LimitReader(r.Body, maxCreateBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.Version != "send.v1" {
		writeError(w, http.StatusBadRequest, "UNSUPPORTED_VERSION", "unsupported send version")
		return
	}
	if len(req.Parts) == 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "a send must declare at least one part")
		return
	}
	if len(req.Parts) > s.cfg.MaxPartCount {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "too many parts")
		return
	}

	ttl := time.Duration(req.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = s.cfg.DefaultTTL
	}
	if ttl > s.cfg.MaxTTL {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "ttl exceeds the server maximum")
		return
	}

	parts := make([]store.PartMeta, 0, len(req.Parts))
	seen := make(map[string]struct{}, len(req.Parts))
	var total int64
	for _, p := range req.Parts {
		if p.PartID == "" || p.SHA256 == "" || p.EncryptedSize <= 0 {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid part declaration")
			return
		}
		if _, dup := seen[p.PartID]; dup {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "duplicate part id")
			return
		}
		seen[p.PartID] = struct{}{}
		if p.EncryptedSize > s.cfg.MaxPartBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "part exceeds the size cap")
			return
		}
		total += p.EncryptedSize
		parts = append(parts, store.PartMeta{PartID: p.PartID, EncryptedSize: p.EncryptedSize, SHA256: p.SHA256})
	}
	if total > s.cfg.MaxTotalBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "total send size exceeds the cap")
		return
	}

	oneTime := true
	if req.OneTime != nil {
		oneTime = *req.OneTime
	}
	rec, err := s.store.CreateSend(r.Context(), store.CreateSendInput{
		Version: req.Version, OneTime: oneTime, TTL: ttl, Parts: parts,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	base := s.publicBase(r)
	uploadURLs := make(map[string]string, len(rec.Parts))
	for _, p := range rec.Parts {
		uploadURLs[p.PartID] = "/v1/sends/" + rec.ID + "/parts/" + p.PartID
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":          rec.ID,
		"upload_urls": uploadURLs,
		"public_url":  base + "/s/" + rec.ID,
		"expires_at":  rec.ExpiresAt.UTC().Format(time.RFC3339),
		"one_time":    rec.OneTime,
	})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if !s.allow(r, "upload") {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
		return
	}
	id := r.PathValue("id")
	partID := r.PathValue("part_id")
	declared := store.PartMeta{PartID: partID, SHA256: r.Header.Get("X-Send-Ciphertext-Sha256")}

	err := s.store.PutPart(r.Context(), id, partID, r.Body, declared)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "part_id": partID, "encrypted_size": max(r.ContentLength, 0)})
}

func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.FinalizeSend(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	meta, err := s.store.GetSendMeta(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"public_url": s.publicBase(r) + "/s/" + id,
		"expires_at": meta.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	meta, err := s.store.GetSendMeta(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	parts := make([]map[string]any, 0, len(meta.Parts))
	for _, p := range meta.Parts {
		parts = append(parts, map[string]any{
			"part_id": p.PartID, "encrypted_size": p.EncryptedSize, "sha256": p.SHA256,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                   meta.ID,
		"status":               string(meta.Status),
		"one_time":             meta.OneTime,
		"expires_at":           meta.ExpiresAt.UTC().Format(time.RFC3339),
		"part_count":           meta.PartCount,
		"total_encrypted_size": meta.TotalEncryptedSize,
		"parts":                parts,
	})
}

func (s *Server) handleRedeem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	grant, err := s.store.RedeemSend(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	parts := make([]partBrief, 0, len(grant.Parts))
	for _, p := range grant.Parts {
		parts = append(parts, partBrief{PartID: p.PartID, EncryptedSize: p.EncryptedSize})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"redeem_token": grant.Token,
		"expires_at":   grant.ExpiresAt.UTC().Format(time.RFC3339),
		"parts":        parts,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	partID := r.PathValue("part_id")
	token := bearerOrQuery(r)

	rc, err := s.store.GetPart(r.Context(), id, partID, token)
	if err != nil {
		// An unknown part on download is a 404, not the upload-time 400.
		if errors.Is(err, store.ErrUnknownPart) {
			writeError(w, http.StatusNotFound, "SEND_NOT_FOUND", "part not found")
			return
		}
		writeStoreError(w, err)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}
