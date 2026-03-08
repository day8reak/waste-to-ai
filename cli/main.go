package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// CLI配置
var (
	serverAddr = flag.String("server", "http://localhost:8080", "server address")
)

// 任务结构
type Task struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Image        string   `json:"image"`
	GPURequired  int      `json:"gpu_required"`
	GPUModel     string   `json:"gpu_model"`
	Priority     int      `json:"priority"`
	Status       string   `json:"status"`
	GPUAssigned  []string `json:"gpu_assigned"`
	CreatedAt   string   `json:"created_at"`
}

type GPUDevice struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Memory int    `json:"memory"`
	Node   string `json:"node"`
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

func main() {
	flag.Parse()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "submit":
		cmdSubmit()
	case "tasks":
		cmdTasks()
	case "task":
		cmdTask()
	case "kill":
		cmdKill()
	case "gpus":
		cmdGPUs()
	case "stats":
		cmdStats()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`GPU Scheduler CLI

Usage:
  gpu-cli <command> [options]

Commands:
  submit [options]   Submit a new task
  tasks              List all tasks
  task <id>          Get task details
  kill <id>          Kill a running task
  gpus               List all GPUs
  stats              Get cluster statistics
  help               Show this help message

Submit Options:
  --name <name>         Task name
  --image <image>       Docker image
  --command <cmd>       Command to run
  --gpu <n>             Number of GPUs required (default: 1)
  --model <model>       Required GPU model (e.g., V100, 3090)
  --priority <n>        Task priority 1-10 (default: 5)

Examples:
  gpu-cli submit --name "llm-test" --image "pytorch/pytorch:2.0" --command "python train.py" --gpu 1
  gpu-cli tasks
  gpu-cli gpus
  gpu-cli kill task123456
`)
}

func cmdSubmit() {
	var name, image, command, model string
	var gpu, priority int

	flag.CommandLine = flag.NewFlagSet("submit", flag.ExitOnError)
	flag.StringVar(&name, "name", "", "task name")
	flag.StringVar(&image, "image", "", "docker image")
	flag.StringVar(&command, "command", "", "command to run")
	flag.IntVar(&gpu, "gpu", 1, "number of GPUs")
	flag.StringVar(&model, "model", "", "GPU model")
	flag.IntVar(&priority, "priority", 5, "priority")
	flag.Parse()

	if image == "" || command == "" {
		fmt.Println("Error: --image and --command are required")
		os.Exit(1)
	}

	if name == "" {
		name = "task-" + time.Now().Format("150405")
	}

	// 构建请求
	reqBody := map[string]interface{}{
		"name":         name,
		"image":       image,
		"command":     command,
		"gpu_required": gpu,
		"gpu_model":   model,
		"priority":    priority,
	}

	// 发送请求
	resp, err := http.Post(*serverAddr+"/api/tasks", "application/json", toJSON(reqBody))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Printf("Error (HTTP %d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// 解析响应
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	fmt.Printf("Task submitted successfully!\n")
	fmt.Printf("  Task ID: %s\n", result["task_id"])
	fmt.Printf("  Status: %s\n", result["status"])
}

func cmdTasks() {
	resp, err := http.Get(*serverAddr + "/api/tasks")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	tasks := result["tasks"].([]interface{})
	fmt.Printf("Total tasks: %d\n\n", len(tasks))

	for _, t := range tasks {
		task := t.(map[string]interface{})
		fmt.Printf("ID: %s\n", task["id"])
		fmt.Printf("  Name: %s\n", task["name"])
		fmt.Printf("  Status: %s\n", task["status"])
		fmt.Printf("  GPUs: %v\n", task["gpu_assigned"])
		fmt.Printf("  Created: %s\n", task["created_at"])
		fmt.Println()
	}
}

func cmdTask() {
	if len(os.Args) < 3 {
		fmt.Println("Error: task ID required")
		fmt.Println("Usage: gpu-cli task <task-id>")
		os.Exit(1)
	}

	id := os.Args[2]
	resp, err := http.Get(*serverAddr + "/api/tasks/" + id)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		fmt.Printf("Task %s not found\n", id)
		os.Exit(1)
	}

	var task Task
	json.NewDecoder(resp.Body).Decode(&task)

	fmt.Printf("Task: %s\n", task.ID)
	fmt.Printf("  Name: %s\n", task.Name)
	fmt.Printf("  Image: %s\n", task.Image)
	fmt.Printf("  Command: %s\n", task.Command)
	fmt.Printf("  Status: %s\n", task.Status)
	fmt.Printf("  GPUs Required: %d\n", task.GPURequired)
	fmt.Printf("  GPUs Assigned: %v\n", task.GPUAssigned)
	fmt.Printf("  Priority: %d\n", task.Priority)
	fmt.Printf("  Created: %s\n", task.CreatedAt)
}

func cmdKill() {
	if len(os.Args) < 3 {
		fmt.Println("Error: task ID required")
		fmt.Println("Usage: gpu-cli kill <task-id>")
		os.Exit(1)
	}

	id := os.Args[2]
	resp, err := http.Post(*serverAddr+"/api/tasks/"+id+"/kill", "application/json", nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Printf("Task %s killed successfully\n", id)
}

func cmdGPUs() {
	resp, err := http.Get(*serverAddr + "/api/gpus")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	gpus := result["gpus"].([]interface{})
	fmt.Printf("Total GPUs: %d\n\n", len(gpus))

	for _, g := range gpus {
		gpu := g.(map[string]interface{})
		taskID := gpu["task_id"]

		fmt.Printf("GPU: %s | %s | %s\n", gpu["id"], gpu["model"], gpu["status"])
		if taskID != "" {
			fmt.Printf("  Task: %s\n", taskID)
		}
	}
}

func cmdStats() {
	resp, err := http.Get(*serverAddr + "/api/stats")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var stats map[string]int
	json.NewDecoder(resp.Body).Decode(&stats)

	fmt.Println("=== Cluster Statistics ===")
	fmt.Printf("Pending:  %d\n", stats["pending"])
	fmt.Printf("Running:  %d\n", stats["running"])
	fmt.Printf("Completed: %d\n", stats["completed"])
	fmt.Printf("Total:    %d\n", stats["pending"]+stats["running"]+stats["completed"])
}

func toJSON(v interface{}) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}
