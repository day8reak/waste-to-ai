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
	handler.RegisterRoutes(router)

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort)
	fmt.Printf("Server starting on http://%s\n", addr)
	fmt.Printf("API Endpoints:\n")
	fmt.Printf("  POST   /api/tasks        - Submit task\n")
	fmt.Printf("  GET    /api/tasks        - List tasks\n")
	fmt.Printf("  GET    /api/tasks/{id}   - Get task\n")
	fmt.Printf("  POST   /api/tasks/{id}/kill - Kill task\n")
	fmt.Printf("  GET    /api/gpus         - List GPUs\n")
	fmt.Printf("  GET    /api/stats        - Get stats\n")
	fmt.Printf("  GET    /health           - Health check\n")

	log.Fatal(http.ListenAndServe(addr, router))
}
