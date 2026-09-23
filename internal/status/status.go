package status

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/honeok/homedash-core/internal/core"
)

// 保存程序启动时间, 用于计算运行时长
type Handler struct {
	startedAt time.Time
}

// 定义 /v1/status 接口响应结构
type runtimeResponse struct {
	Version    string        `json:"version"`
	GitCommit  string        `json:"gitCommit"`
	BuildTime  string        `json:"buildTime"`
	GoVersion  string        `json:"goVersion"`
	Goroutines int           `json:"goroutines"`
	Memory     runtimeMemory `json:"memory"`
	Uptime     int64         `json:"uptime"`
}

// 定义需要对外暴露的 Go 内存统计信息
type runtimeMemory struct {
	HeapAlloc  uint64 `json:"heapAlloc"`
	HeapInuse  uint64 `json:"heapInuse"`
	StackInuse uint64 `json:"stackInuse"`
	Sys        uint64 `json:"sys"`
}

// 记录程序开始提供服务的时间
func NewHandler() *Handler {
	return &Handler{startedAt: time.Now()}
}

// 返回当前程序的构建信息和 Go Runtime 状态
func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(runtimeResponse{
		Version:    core.Version,
		GitCommit:  core.GitCommit,
		BuildTime:  core.BuildTime,
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		Memory: runtimeMemory{
			HeapAlloc:  mem.HeapAlloc,
			HeapInuse:  mem.HeapInuse,
			StackInuse: mem.StackInuse,
			Sys:        mem.Sys,
		},
		Uptime: int64(time.Since(h.startedAt) / time.Second),
	}); err != nil {
		http.Error(w, "failed to encode runtime response", http.StatusInternalServerError)
	}
}
