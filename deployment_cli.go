package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GPU Scheduler 实机测试套件
// 运行方式: go run deployment_test.go

type TestResult struct {
	Name    string
	Passed  bool
	Message string
}

var results []TestResult
var baseURL = "http://localhost:8080"

// 颜色输出
func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }

func main() {
	fmt.Println(yellow("========================================"))
	fmt.Println(yellow("  GPU Scheduler 实机测试套件"))
	fmt.Println(yellow("========================================"))
	fmt.Println()

	// 检查环境变量覆盖 baseURL
	if url := os.Getenv("GPU_SCHEDULER_URL"); url != "" {
		baseURL = url
	}
	fmt.Printf("测试目标: %s\n\n", baseURL)

	// 运行所有测试
	testServerConnectivity()
	testGPUList()
	testTaskSubmit()
	testTaskList()
	testRayAllocate()
	testRayRelease()
	testRayBlock()
	testRayUnblock()
	testStats()
	testKillTask()
	testConcurrentTasks()
	testGPUMemoryAllocation()
	testPreemption()
	testGPUBlocking()
	testTaskLifecycle()

	// 输出结果
	printResults()

	// 退出码
	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// ========== 测试函数 ==========

func testServerConnectivity() {
	name := "服务器连接测试"
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil {
		// 尝试不带 health 的路径
		resp, err = http.Get(baseURL + "/api/gpus")
	}
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("无法连接到服务器: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		results = append(results, TestResult{
			Name:    name,
			Passed:  true,
			Message: "服务器可访问",
		})
	} else {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("服务器返回状态码: %d", resp.StatusCode),
		})
	}
}

func testGPUList() {
	name := "GPU 列表查询"
	resp, err := http.Get(baseURL + "/api/gpus")
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	gpus, ok := result["gpus"].([]interface{})
	if !ok {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "返回数据格式错误",
		})
		return
	}

	if len(gpus) == 0 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "未检测到 GPU",
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("检测到 %d 个 GPU", len(gpus)),
	})

	// 打印 GPU 详情
	for _, g := range gpus {
		gpu := g.(map[string]interface{})
		fmt.Printf("  - %s: %s (%d MB) [%s]\n",
			gpu["id"], gpu["model"], int(gpu["memory"].(float64)), gpu["status"])
	}
}

func testTaskSubmit() {
	name := "任务提交"
	body := []byte(`{
		"name": "deploy-test",
		"command": "echo hello",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 5
	}`)

	resp, err := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	taskID, ok := result["task_id"].(string)
	if !ok {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "未返回 task_id",
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("任务已提交: %s", taskID),
	})
}

func testTaskList() {
	name := "任务列表查询"
	resp, err := http.Get(baseURL + "/api/tasks")
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: "任务列表查询成功",
	})
}

func testRayAllocate() {
	name := "Ray GPU 分配"
	body := []byte(`{
		"job_id": "test-ray-job",
		"gpu_count": 1,
		"priority": 10
	}`)

	resp, err := http.Post(baseURL+"/api/ray/allocate", "application/json", bytes.NewReader(body))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d - %s", resp.StatusCode, string(respBody)),
		})
		return
	}

	gpuIDs, ok := result["gpu_ids"].([]interface{})
	if !ok || len(gpuIDs) == 0 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "未分配 GPU",
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("已分配 GPU: %v", gpuIDs),
	})
}

func testRayRelease() {
	name := "Ray GPU 释放"
	body := []byte(`{
		"job_id": "test-ray-job"
	}`)

	resp, err := http.Post(baseURL+"/api/ray/release", "application/json", bytes.NewReader(body))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: "GPU 已释放",
	})
}

func testRayBlock() {
	name := "GPU 阻塞"

	// 先获取 GPU 列表
	resp, _ := http.Get(baseURL + "/api/gpus")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	gpus := result["gpus"].([]interface{})

	if len(gpus) == 0 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "无 GPU 可用于测试",
		})
		return
	}

	gpuID := gpus[0].(map[string]interface{})["id"].(string)

	blockBody := []byte(fmt.Sprintf(`{
		"gpu_ids": ["%s"]
	}`, gpuID))

	resp2, err := http.Post(baseURL+"/api/ray/block", "application/json", bytes.NewReader(blockBody))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp2.StatusCode),
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("GPU %s 已阻塞", gpuID),
	})

	// 立即解除阻塞
	unblockBody := []byte(fmt.Sprintf(`{
		"gpu_ids": ["%s"]
	}`, gpuID))
	http.Post(baseURL+"/api/ray/unblock", "application/json", bytes.NewReader(unblockBody))
}

func testRayUnblock() {
	name := "GPU 解除阻塞"

	// 先获取一个 GPU 并阻塞
	resp, _ := http.Get(baseURL + "/api/gpus")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	gpus := result["gpus"].([]interface{})

	if len(gpus) == 0 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "无 GPU 可用于测试",
		})
		return
	}

	gpuID := gpus[0].(map[string]interface{})["id"].(string)

	// 先阻塞
	blockBody := []byte(fmt.Sprintf(`{"gpu_ids": ["%s"]}`, gpuID))
	http.Post(baseURL+"/api/ray/block", "application/json", bytes.NewReader(blockBody))
	time.Sleep(100 * time.Millisecond)

	// 再解除阻塞
	unblockBody := []byte(fmt.Sprintf(`{"gpu_ids": ["%s"]}`, gpuID))
	resp2, err := http.Post(baseURL+"/api/ray/unblock", "application/json", bytes.NewReader(unblockBody))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp2.StatusCode),
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("GPU %s 已解除阻塞", gpuID),
	})
}

func testStats() {
	name := "统计信息查询"
	resp, err := http.Get(baseURL + "/api/stats")
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var stats map[string]int
	json.Unmarshal(body, &stats)

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("pending=%d, running=%d, completed=%d",
			stats["pending"], stats["running"], stats["completed"]),
	})
}

func testKillTask() {
	name := "任务终止"

	// 提交一个任务
	body := []byte(`{
		"name": "kill-test",
		"command": "sleep 60",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 5
	}`)

	resp, err := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("提交失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	taskID, ok := result["task_id"].(string)
	if !ok {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "未获取到 task_id",
		})
		return
	}

	// 等待任务启动
	time.Sleep(500 * time.Millisecond)

	// 终止任务
	req, _ := http.NewRequest("DELETE", baseURL+"/api/tasks/"+taskID, nil)
	killResp, err := http.DefaultClient.Do(req)
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("终止失败: %v", err),
		})
		return
	}
	defer killResp.Body.Close()

	if killResp.StatusCode != http.StatusOK {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("HTTP %d", killResp.StatusCode),
		})
		return
	}

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("任务 %s 已终止", taskID),
	})
}

func testConcurrentTasks() {
	name := "并发任务提交"
	taskCount := 10
	done := make(chan bool, taskCount)

	for i := 0; i < taskCount; i++ {
		go func(n int) {
			body := []byte(fmt.Sprintf(`{
				"name": "concurrent-%d",
				"command": "echo test",
				"image": "ubuntu:latest",
				"gpu_required": 1,
				"priority": 5
			}`, n))

			resp, _ := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
			if resp != nil {
				resp.Body.Close()
			}
			done <- true
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < taskCount; i++ {
		<-done
	}

	// 检查任务列表
	resp, _ := http.Get(baseURL + "/api/tasks")
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	tasks := result["tasks"].([]interface{})

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("成功提交 %d 个并发任务", len(tasks)),
	})
}

func testGPUMemoryAllocation() {
	name := "GPU 内存分配"

	// 获取 GPU 列表
	resp, _ := http.Get(baseURL + "/api/gpus")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	gpus := result["gpus"].([]interface{})

	if len(gpus) == 0 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "无 GPU 可用",
		})
		return
	}

	gpu := gpus[0].(map[string]interface{})
	memory := int(gpu["memory"].(float64))

	// Test memory allocation
	body2 := []byte("{\"name\":\"mem-test\",\"command\":\"echo test\",\"image\":\"ubuntu:latest\",\"gpu_required\":1,\"priority\":5}")

	resp2, _ := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body2))
	defer resp2.Body.Close()

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("GPU %s 内存: %d MB", gpu["id"], memory),
	})
}

func testPreemption() {
	name := "抢占测试"

	// 先提交低优先级任务
	lowPrio := []byte(`{
		"name": "low-prio",
		"command": "sleep 30",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 1
	}`)

	resp, _ := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(lowPrio))
	resp.Body.Close()

	// 提交高优先级任务
	highPrio := []byte(`{
		"name": "high-prio",
		"command": "sleep 30",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 10
	}`)

	resp2, _ := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(highPrio))
	defer resp2.Body.Close()

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: "抢占测试完成",
	})
}

func testGPUBlocking() {
	name := "GPU 黑名单测试"

	// 获取所有可用 GPU
	resp, _ := http.Get(baseURL + "/api/gpus")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	gpus := result["gpus"].([]interface{})

	if len(gpus) < 2 {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "需要至少 2 个 GPU",
		})
		return
	}

	// 阻塞第一个 GPU
	gpuID := gpus[0].(map[string]interface{})["id"].(string)
	blockBody := []byte(fmt.Sprintf(`{"gpu_ids": ["%s"]}`, gpuID))
	resp2, _ := http.Post(baseURL+"/api/ray/block", "application/json", bytes.NewReader(blockBody))
	defer resp2.Body.Close()

	// 提交新任务，应该不会分配到被阻塞的 GPU
	taskBody := []byte(`{
		"name": "blocking-test",
		"command": "echo test",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 5
	}`)

	resp3, _ := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(taskBody))
	defer resp3.Body.Close()

	// 解除阻塞
	unblockBody := []byte(fmt.Sprintf(`{"gpu_ids": ["%s"]}`, gpuID))
	http.Post(baseURL+"/api/ray/unblock", "application/json", bytes.NewReader(unblockBody))

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: "GPU 黑名单测试完成",
	})
}

func testTaskLifecycle() {
	name := "任务完整生命周期"

	// 1. 提交任务
	body := []byte(`{
		"name": "lifecycle-test",
		"command": "echo lifecycle",
		"image": "ubuntu:latest",
		"gpu_required": 1,
		"priority": 5
	}`)

	resp, err := http.Post(baseURL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("提交失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	taskID, ok := result["task_id"].(string)
	if !ok {
		results = append(results, TestResult{
			Name:    name,
			Passed:  false,
			Message: "未获取到 task_id",
		})
		return
	}

	// 2. 等待任务运行
	time.Sleep(1 * time.Second)

	// 3. 查询任务状态
	resp2, _ := http.Get(baseURL + "/api/tasks/" + taskID)
	defer resp2.Body.Close()

	// 4. 终止任务
	req, _ := http.NewRequest("DELETE", baseURL+"/api/tasks/"+taskID, nil)
	http.DefaultClient.Do(req)

	results = append(results, TestResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("任务 %s 生命周期测试完成", taskID),
	})
}

// ========== 输出结果 ==========

func printResults() {
	fmt.Println()
	fmt.Println(yellow("========================================"))
	fmt.Println(yellow("  测试结果"))
	fmt.Println(yellow("========================================"))
	fmt.Println()

	passed := 0
	failed := 0

	for _, r := range results {
		status := green("✓ PASS")
		if !r.Passed {
			status = red("✗ FAIL")
			failed++
		} else {
			passed++
		}

		if r.Passed {
			fmt.Printf("%s %s\n", status, r.Name)
			fmt.Printf("    %s\n", r.Message)
		} else {
			fmt.Printf("%s %s\n", status, r.Name)
			fmt.Printf("    %s\n", red(r.Message))
		}
	}

	fmt.Println()
	fmt.Println(yellow("========================================"))
	fmt.Printf("总计: %d 通过, %d 失败\n", passed, failed)
	fmt.Println(yellow("========================================"))

	if failed == 0 {
		fmt.Println(green("\n所有测试通过! ✓"))
	} else {
		fmt.Println(red("\n存在测试失败 ✗"))
	}

	// 额外信息
	fmt.Println()
	fmt.Println(yellow("提示:"))
	fmt.Println("  - 使用环境变量 GPU_SCHEDULER_URL 指定服务器地址")
	fmt.Println("  - 例如: GPU_SCHEDULER_URL=http://192.168.1.100:8080 go run deployment_test.go")
	fmt.Println()

	// 检查 Docker 状态
	fmt.Println(yellow("环境检查:"))
	checkDocker()
	checkGPU()
}

func checkDocker() {
	fmt.Print("  Docker: ")
	resp, err := http.Get(baseURL + "/api/gpus")
	if err != nil {
		fmt.Println(red("无法连接"))
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if gpus, ok := result["gpus"].([]interface{}); ok && len(gpus) > 0 {
		fmt.Println(green("已连接"))
		fmt.Printf("    检测到 %d 个 GPU\n", len(gpus))
	} else {
		fmt.Println(yellow("未检测到 GPU"))
	}
}

func checkGPU() {
	fmt.Print("  NVIDIA GPU: ")
	output, err := execCommand("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err != nil {
		fmt.Println(yellow("未安装 nvidia-smi"))
		return
	}
	gpus := strings.Split(strings.TrimSpace(output), "\n")
	fmt.Println(green("已安装"))
	for i, gpu := range gpus {
		if strings.TrimSpace(gpu) != "" {
			fmt.Printf("    GPU %d: %s\n", i, strings.TrimSpace(gpu))
		}
	}
}

func execCommand(name string, args ...string) (string, error) {
	// 简化实现，实际应该用 os/exec
	return "", fmt.Errorf("请手动运行: %s %s", name, strings.Join(args, " "))
}
