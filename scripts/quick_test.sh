#!/bin/bash
# GPU Scheduler 快速测试脚本
# 使用方法: ./quick_test.sh [服务器地址]

URL="${1:-http://localhost:8080}"

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "  GPU Scheduler 快速测试"
echo "========================================"
echo "服务器: $URL"
echo ""

# 测试函数
test() {
    name=$1
    cmd=$2
    echo -n "  $name... "
    if eval "$cmd" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        return 1
    fi
}

# 运行测试
passed=0
failed=0

test "服务器连接" "curl -s -f $URL/api/gpus" && ((passed++)) || ((failed++))
test "GPU列表" "curl -s $URL/api/gpus | grep -q gpu" && ((passed++)) || ((failed++))
test "任务提交" "curl -s -X POST $URL/api/tasks -H 'Content-Type: application/json' -d '{\"name\":\"test\",\"command\":\"echo\",\"image\":\"ubuntu\",\"gpu_required\":1}' | grep -q task_id" && ((passed++)) || ((failed++))
test "Ray分配" "curl -s -X POST $URL/api/ray/allocate -H 'Content-Type: application/json' -d '{\"job_id\":\"test\",\"gpu_count\":1}' | grep -q gpu_ids" && ((passed++)) || ((failed++))
test "Ray释放" "curl -s -X POST $URL/api/ray/release -H 'Content-Type: application/json' -d '{\"job_id\":\"test\"}' | grep -q released" && ((passed++)) || ((failed++))
test "统计信息" "curl -s $URL/api/stats | grep -q pending" && ((passed++)) || ((failed++))

echo ""
echo "========================================"
echo "结果: $passed 通过, $failed 失败"
echo "========================================"

[ $failed -eq 0 ]
