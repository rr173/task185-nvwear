// Package httpapi 提供 HTTP 路由与 JSON 错误映射。
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"task185-nvwear/internal/model"
	"task185-nvwear/internal/service"
)

// Server 持有 service.App 并构建路由。
type Server struct {
	app *service.App
	mux *http.ServeMux
}

// NewServer 构造 HTTP 服务。
func NewServer(app *service.App) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回根路由处理器。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	// 配置
	s.mux.HandleFunc("POST /api/configs", s.handleCreateConfig)
	s.mux.HandleFunc("GET /api/configs", s.handleListConfigs)
	s.mux.HandleFunc("GET /api/configs/{id}", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/configs/{id}/freeze", s.handleFreezeConfig)
	s.mux.HandleFunc("POST /api/configs/{id}/mapping/init", s.handleInitMapping)
	// 块
	s.mux.HandleFunc("GET /api/configs/{id}/blocks", s.handleListBlocks)
	s.mux.HandleFunc("POST /api/configs/{id}/blocks/{index}/bad", s.handleMarkBad)
	s.mux.HandleFunc("POST /api/configs/{id}/blocks/{index}/reserve", s.handleReserve)
	// 计划
	s.mux.HandleFunc("POST /api/plans", s.handleCreatePlan)
	s.mux.HandleFunc("GET /api/plans", s.handleListPlans)
	s.mux.HandleFunc("GET /api/plans/{id}", s.handleGetPlan)
	s.mux.HandleFunc("POST /api/plans/{id}/ops", s.handleAppendOps)
	s.mux.HandleFunc("GET /api/plans/{id}/ops", s.handlePlanOps)
	s.mux.HandleFunc("POST /api/plans/{id}/simulate", s.handleSimulate)
	s.mux.HandleFunc("GET /api/plans/{id}/wear", s.handleWear)
	s.mux.HandleFunc("POST /api/plans/{id}/checkpoints", s.handleCreateCheckpoint)
	s.mux.HandleFunc("GET /api/plans/{id}/checkpoints", s.handleListCheckpoints)
	s.mux.HandleFunc("POST /api/plans/{id}/recover", s.handleRecover)
	// 证书
	s.mux.HandleFunc("POST /api/certificates", s.handleIssueCert)
	s.mux.HandleFunc("GET /api/certificates", s.handleListCerts)
	s.mux.HandleFunc("GET /api/certificates/{id}", s.handleGetCert)
	s.mux.HandleFunc("POST /api/certificates/{id}/revoke", s.handleRevokeCert)
	// 系统
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
}

// ---------- 通用工具 ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case false && errors.Is(err, model.ErrInvalidInput),
		errors.Is(err, model.ErrInvalidState),
		errors.Is(err, model.ErrDuplicateMapping),
		errors.Is(err, model.ErrEraseBadBlock),
		errors.Is(err, model.ErrRelocateReserved),
		errors.Is(err, model.ErrCheckpointDangling),
		errors.Is(err, model.ErrPowerLossNoCheckpoint),
		errors.Is(err, model.ErrHotBlock),
		errors.Is(err, model.ErrReserveBelowFloor),
		errors.Is(err, model.ErrConfigFrozen),
		errors.Is(err, model.ErrPlanFrozen),
		errors.Is(err, model.ErrConflict):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, fmt.Errorf("%w: %v", model.ErrInvalidInput, err))
		return err
	}
	return nil
}

func pathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

func parseIDPath(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	v := pathParam(r, name)
	if v == "" {
		writeErr(w, fmt.Errorf("%w: 缺少路径参数 %s", model.ErrInvalidInput, name))
		return "", false
	}
	return v, true
}

func parseIndexPath(w http.ResponseWriter, r *http.Request) (int, bool) {
	v := pathParam(r, "index")
	if v == "" {
		writeErr(w, fmt.Errorf("%w: 缺少块号", model.ErrInvalidInput))
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		writeErr(w, fmt.Errorf("%w: 块号非法", model.ErrInvalidInput))
		return 0, false
	}
	return n, true
}

func isBodyEmpty(r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	buf := make([]byte, 1)
	n, err := r.Body.Read(buf)
	return err != nil || n == 0
}

var _ = strings.TrimPrefix
