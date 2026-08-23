package httpapi

import (
	"net/http"
)

// handleCreateCheckpoint 创建检查点。
func (s *Server) handleCreateCheckpoint(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Cursor int `json:"cursor"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	cp, err := s.app.Plans.Checkpoint(r.Context(), id, req.Cursor)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cp)
}

// handleListCheckpoints 列出检查点。
func (s *Server) handleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	list, err := s.app.Plans.Checkpoints(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleRecover 执行掉电恢复验证。
func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	rep, err := s.app.Plans.Recover(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
