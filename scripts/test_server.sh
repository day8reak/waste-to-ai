#!/bin/bash
# GPU Scheduler 实机测试脚本
# 使用方法: ./test_server.sh [服务器地址]

set -e

# 配置
SERVER_URL="${1:-http://localhost:8080}"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}========================================"
echo -e "  GPU Scheduler 实机测试"
echo -e "========================================${NC}"
echo ""
echo "测试目标: $SERVER_URL"
echo ""

# 测试函数
test_server() {
    echo -n "测试服务器连接... "
    if curl -s -f "$SERVER_URL/api/gpus" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        return 1
    fi
}

test_gpu_list() {
    echo -e "\n${YELLOW}测试 1: GPU 列表查询${NC}"
    echo "----------------------------------------"
    curl -s "$SERVER_URL/api/gpus" | python3 -m json.tool 2>/dev/null || \
    curl -s "$SERVER_URL/api/gpus"
}

test_task_submit() {
    echo -e "\n${YELLOW}测试 2: 任务提交${NC}"
    echo "----------------------------------------"
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/tasks" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "deploy-test",
            "command": "echo hello",
            "image": "ubuntu:latest",
            "gpu_required": 1,
            "priority": 5
        }')
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
    echo -e "${GREEN}任务已提交${NC}"
}

test_task_list() {
    echo -e "\n${YELLOW}测试 3: 任务列表${NC}"
    echo "----------------------------------------"
    curl -s "$SERVER_URL/api/tasks" | python3 -m json.tool 2>/dev/null || \
    curl -s "$SERVER_URL/api/tasks"
}

test_ray_allocate() {
    echo -e "\n${YELLOW}测试 4: Ray GPU 分配${NC}"
    echo "----------------------------------------"
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/ray/allocate" \
        -H "Content-Type: application/json" \
        -d '{
            "job_id": "test-ray-job",
            "gpu_count": 1,
            "priority": 10
        }')
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
}

test_ray_release() {
    echo -e "\n${YELLOW}测试 5: Ray GPU 释放${NC}"
    echo "----------------------------------------"
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/ray/release" \
        -H "Content-Type: application/json" \
        -d '{"job_id": "test-ray-job"}')
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
    echo -e "${GREEN}GPU 已释放${NC}"
}

test_ray_block() {
    echo -e "\n${YELLOW}测试 6: GPU 阻塞${NC}"
    echo "----------------------------------------"
    # 获取第一个 GPU ID
    GPU_ID=$(curl -s "$SERVER_URL/api/gpus" | python3 -c "import sys,json; print(json.load(sys.stdin)['gpus'][0]['id'])" 2>/dev/null)

    if [ -z "$GPU_ID" ]; then
        echo "无法获取 GPU ID，跳过测试"
        return
    fi

    echo "阻塞 GPU: $GPU_ID"
    RESPONSE=$(curl -s -X POST "$SERVER_URL/api/ray/block" \
        -H "Content-Type: application/json" \
        -d "{\"gpu_ids\": [\"$GPU_ID\"]}")
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"

    # 立即解除
    sleep 0.5
    curl -s -X POST "$SERVER_URL/api/ray/unblock" \
        -H "Content-Type: application/json" \
        -d "{\"gpu_ids\": [\"$GPU_ID\"]}" > /dev/null
    echo -e "${GREEN}GPU 已解除阻塞${NC}"
}

test_stats() {
    echo -e "\n${YELLOW}测试 7: 统计信息${NC}"
    echo "----------------------------------------"
    curl -s "$SERVER_URL/api/stats" | python3 -m json.tool 2>/dev/null || \
    curl -s "$SERVER_URL/api/stats"
}

test_kill_task() {
    echo -e "\n${YELLOW}测试 8: 任务终止${NC}"
    echo "----------------------------------------"

    # 提交任务
    TASK_RESP=$(curl -s -X POST "$SERVER_URL/api/tasks" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "kill-test",
            "command": "sleep 60",
            "image": "ubuntu:latest",
            "gpu_required": 1,
            "priority": 5
        }')

    TASK_ID=$(echo "$TASK_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('task_id',''))" 2>/dev/null)

    if [ -z "$TASK_ID" ]; then
        echo "无法获取 task_id"
        return
    fi

    echo "任务 ID: $TASK_ID"
    sleep 1

    # 终止任务
    curl -s -X DELETE "$SERVER_URL/api/tasks/$TASK_ID"
    echo -e "\n${GREEN}任务已终止${NC}"
}

test_concurrent() {
    echo -e "\n${YELLOW}测试 9: 并发任务提交${NC}"
    echo "----------------------------------------"

    for i in {1..5}; do
        curl -s -X POST "$SERVER_URL/api/tasks" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"concurrent-$i\",
                \"command\": \"echo test\",
                \"image\": \"ubuntu:latest\",
                \"gpu_required\": 1,
                \"priority\": 5
            }" &
    done

    wait
    echo -e "${GREEN}并发任务提交完成${NC}"
}

test_health() {
    echo -e "\n${YELLOW}测试 10: 健康检查${NC}"
    echo "----------------------------------------"
    curl -s -w "\nHTTP Status: %{http_code}\n" "$SERVER_URL/api/health" || \
    curl -s -w "\nHTTP Status: %{http_code}\n" "$SERVER_URL/api/gpus"
}

# 运行测试
run_tests() {
    if ! test_server; then
        echo -e "${RED}服务器连接失败，请检查服务器是否运行${NC}"
        echo "启动服务器: go run cmd/main.go"
        exit 1
    fi

    test_gpu_list
    test_task_submit
    test_task_list
    test_ray_allocate
    test_ray_release
    test_ray_block
    test_stats
    test_kill_task
    test_concurrent
    test_health

    echo -e "\n${GREEN}========================================"
    echo -e "  所有测试完成"
    echo -e "========================================${NC}"
}

# 主程序
case "${2:-run}" in
    gpu)
        test_gpu_list
        ;;
    task)
        test_task_submit
        test_task_list
        ;;
    ray)
        test_ray_allocate
        test_ray_release
        test_ray_block
        ;;
    stats)
        test_stats
        ;;
    run)
        run_tests
        ;;
    *)
        echo "用法: $0 [服务器地址] [测试类型]"
        echo ""
        echo "测试类型:"
        echo "  run    - 运行所有测试 (默认)"
        echo "  gpu    - 仅测试 GPU 列表"
        echo "  task   - 仅测试任务提交"
        echo "  ray    - 仅测试 Ray 集成"
        echo "  stats  - 仅测试统计信息"
        echo ""
        echo "示例:"
        echo "  $0 http://localhost:8080 run"
        echo "  $0 http://192.168.1.100:8080 gpu"
        ;;
esac
