package httpapi

import (
	"context"
	"net/http"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/service"
)

// planOpsRequest 创建计划请求体。
type planOpsRequest struct {
	ConfigID string                `json:"config_id"`
	Name     string                `json:"name"`
	Ops      []model.PlanOperation `json:"ops"`
}

// handleCreatePlan 创建操作计划。
func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req planOpsRequest
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	p, err := s.app.Plans.Create(r.Context(), service.PlanCreateInput{
		ConfigID: req.ConfigID, Name: req.Name, Ops: req.Ops,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// handleListPlans 列出配置下计划。
func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	configID := r.URL.Query().Get("config_id")
	if configID == "" {
		writeErr(w, model.ErrInvalidInput)
		return
	}
	list, err := s.app.Plans.List(r.Context(), configID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetPlan 读取计划。
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	p, err := s.app.Plans.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handleAppendOps 追加操作。
func (s *Server) handleAppendOps(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Ops []model.PlanOperation `json:"ops"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		return
	}
	p, err := s.app.Plans.AppendOps(r.Context(), id, req.Ops)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handlePlanOps 读取计划操作序列。
func (s *Server) handlePlanOps(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	ops, err := s.app.Plans.Ops(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ops)
}

// handleSimulate 执行模拟。
func (s *Server) handleSimulate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	p, err := s.app.Plans.Simulate(context.Background(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handleWear 读取磨损曲线。
func (s *Server) handleWear(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	entries, err := s.app.Plans.Wear(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
