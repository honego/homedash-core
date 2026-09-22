package media

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/honeok/homepage-core/internal/config"
	"github.com/honeok/homepage-core/internal/media/game"
)

// 媒体接口
type Handler struct {
	steam *game.Steam
}

// 创建媒体处理器
func NewHandler(cfg config.MediaConfig) *Handler {
	return &Handler{steam: game.NewSteam(cfg.Steam)}
}

// 返回聚合媒体数据
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	steam, err := h.steam.Get(r.Context())
	if err != nil {
		slog.Error("Failed to fetch Steam data", "err", err)
		writeError(w, http.StatusBadGateway, "failed to fetch Steam data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"game": map[string]any{"steam": steam},
	})
}

// 输出错误 JSON
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// 输出 JSON
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
