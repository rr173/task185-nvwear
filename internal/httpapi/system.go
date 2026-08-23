package httpapi

import (
	"net/http"

	"task185-nvwear/internal/model"
)

// handleHealth 健康检查。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "task185-nvwear"})
}

// handleStats 统计汇总。
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.app.Stats.Collect(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// modelErrMissingPlanID 缺少必填参数的领域错误。
func modelErrMissingPlanID() error {
	return model.ErrInvalidInput
}
