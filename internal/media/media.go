package media

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/honeok/homepage-core/internal/config"
	"github.com/honeok/homepage-core/internal/media/game"
	"github.com/honeok/homepage-core/internal/media/music"
	"golang.org/x/sync/errgroup"
)

// 媒体处理器
type Handler struct {
	steam   *game.Steam
	netease *music.NetEase
}

// 创建媒体处理器
func NewHandler(cfg config.MediaConfig) *Handler {
	return &Handler{
		steam:   game.NewSteam(cfg.Steam),
		netease: music.NewNetEase(cfg.Music.NetEase),
	}
}

// 返回聚合媒体数据
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	group, ctx := errgroup.WithContext(r.Context())
	var steam game.Data
	var netease music.NetEaseData
	group.Go(func() error {
		var err error
		steam, err = h.steam.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch Steam data: %w", err)
		}
		return nil
	})
	group.Go(func() error {
		var err error
		netease, err = h.netease.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch NetEase data: %w", err)
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		slog.Error("Failed to fetch media data", "err", err)
		writeError(w, http.StatusBadGateway, "failed to fetch media data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"game":  map[string]any{"steam": steam},
		"music": map[string]any{"netease": netease},
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
