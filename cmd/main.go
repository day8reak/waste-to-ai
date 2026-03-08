package main

import (
	"flag"
	"fmt"
	"gpu-scheduler/internal/api"
	"gpu-scheduler/internal/config"
	"gpu-scheduler/internal/docker"
	"gpu-scheduler/internal/gpu"
	"gpu-scheduler/internal/scheduler"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.json", "config file path")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Failed to load config: %v, using default", err)
		cfg = config.DefaultConfig()
	}

	// 打印配置信息
	fmt.Printf("=== GPU Scheduler ===\n")
	fmt.Printf("Mock Mode: %v\n", cfg.MockMode)
	fmt.Printf("Server: %s:%d\n", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("Preempt: %v\n", cfg.PreemptEnabled)
	fmt.Println("===================")

	// 创建组件
	gpuMgr := gpu.NewGPUManager(cfg)
	dockerMgr := docker.NewDockerManager(cfg.DockerEndpoint, cfg.MockMode)

	// 创建调度器
	sched := scheduler.NewScheduler(gpuMgr, dockerMgr, cfg.PreemptEnabled)

	// 创建API处理器
	handler := api.NewHandler(sched, gpuMgr)

	// 创建路由
	router := mux.NewRouter()

	// 先注册静态文件服务（确保优先匹配）
	webDir := "web"
	if _, err := os.Stat(webDir); err == nil {
		// 根路径服务 index.html
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "web/index.html")
		})

		// 静态文件服务
		staticDir := http.Dir(webDir + "/static")
		staticHandler := http.StripPrefix("/static/", http.FileServer(staticDir))
		router.PathPrefix("/static/").Handler(staticHandler)

		log.Printf("Static file server enabled, serving from: %s", staticDir)
	}

	// 注册 API 路由
	handler.RegisterRoutes(router)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("Server starting on http://%s\n", addr)
	fmt.Printf("API Endpoints:\n")
	fmt.Printf("  GET    /                - Web Admin UI\n")
	fmt.Printf("  POST   /api/tasks        - Submit task\n")
	fmt.Printf("  GET    /api/tasks        - List tasks\n")
	fmt.Printf("  GET    /api/tasks/{id}   - Get task\n")
	fmt.Printf("  POST   /api/tasks/{id}/kill - Kill task\n")
	fmt.Printf("  GET    /api/gpus         - List GPUs\n")
	fmt.Printf("  GET    /api/stats        - Get stats\n")
	fmt.Printf("  GET    /health           - Health check\n")
	fmt.Printf("  POST   /api/ray/allocate - Ray allocate GPU\n")
	fmt.Printf("  POST   /api/ray/release  - Ray release GPU\n")
	fmt.Printf("  GET    /api/ray/status  - Ray status\n")
	fmt.Printf("  POST   /api/ray/block   - Block GPU\n")
	fmt.Printf("  POST   /api/ray/unblock - Unblock GPU\n")

	log.Fatal(http.ListenAndServe(addr, router))
}
