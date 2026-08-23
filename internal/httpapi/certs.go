package httpapi

import (
	"net/http"
)

// handleIssueCert 发布策略证书。
func (s *Server) handleIssueCert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
		Name   string `json:"name"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	if req.PlanID == "" {
		writeErr(w, modelErrMissingPlanID())
		return
	}
	cert, err := s.app.Certs.Issue(r.Context(), req.PlanID, req.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cert)
}

// handleListCerts 列出配置下证书。
func (s *Server) handleListCerts(w http.ResponseWriter, r *http.Request) {
	configID := r.URL.Query().Get("config_id")
	if configID == "" {
		writeErr(w, modelErrMissingPlanID())
		return
	}
	list, err := s.app.Certs.List(r.Context(), configID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetCert 读取证书。
func (s *Server) handleGetCert(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	cert, err := s.app.Certs.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cert)
}

// handleRevokeCert 撤销证书。
func (s *Server) handleRevokeCert(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	cert, err := s.app.Certs.Revoke(r.Context(), id, req.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cert)
}
