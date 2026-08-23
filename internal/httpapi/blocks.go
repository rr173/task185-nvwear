package httpapi

import (
	"net/http"
)

// handleListBlocks 列出配置全部物理块。
func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	blocks, err := s.app.Configs.Blocks(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, blocks)
}

// handleMarkBad 登记坏块。
func (s *Server) handleMarkBad(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	index, ok := parseIndexPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !isBodyEmpty(r) {
		if err := decodeBody(w, r, &body); err != nil {
			return
		}
	}
	b, err := s.app.Configs.MarkBad(r.Context(), id, index, body.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleReserve 设置保留块。
func (s *Server) handleReserve(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	index, ok := parseIndexPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Reserved bool `json:"reserved"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return
	}
	b, err := s.app.Configs.Reserve(r.Context(), id, index, body.Reserved)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}
