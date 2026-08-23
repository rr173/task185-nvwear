package httpapi

import (
	"net/http"

	"task185-nvwear/internal/service"
)

// handleCreateConfig 创建存储配置。
func (s *Server) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	var in service.ConfigCreateInput
	if err := decodeBody(w, r, &in); err != nil {
		return
	}
	cfg, err := s.app.Configs.Create(r.Context(), in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cfg)
}

// handleListConfigs 列出配置。
func (s *Server) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.Configs.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleGetConfig 读取配置详情。
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	cfg, err := s.app.Configs.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleFreezeConfig 冻结配置。
func (s *Server) handleFreezeConfig(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	cfg, err := s.app.Configs.Freeze(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handleInitMapping 初始化逻辑映射。
func (s *Server) handleInitMapping(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDPath(w, r, "id")
	if !ok {
		return
	}
	n, err := s.app.Configs.InitMapping(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config_id": id, "mapping_entries": n})
}
