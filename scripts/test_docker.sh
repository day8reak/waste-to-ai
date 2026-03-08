#!/bin/bash
# GPU Scheduler Docker 部署测试
# 使用方法: ./test_docker.sh

set -e

echo "========================================"
echo "  GPU Scheduler Docker 部署测试"
echo "========================================"

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "错误: Docker 未安装"
    exit 1
fi

# 检查 docker-compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "错误: docker-compose 未安装"
    exit 1
fi

# 配置
IMAGE_NAME="gpu-scheduler:latest"
CONTAINER_NAME="gpu-scheduler-test"
PORT=8080

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${YELLOW}[INFO]${NC} $1"; }
log_ok() { echo -e "${GREEN}[OK]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

cleanup() {
    log_info "清理容器..."
    docker stop $CONTAINER_NAME 2>/dev/null || true
    docker rm $CONTAINER_NAME 2>/dev/null || true
}

# 构建镜像
build_image() {
    log_info "构建 Docker 镜像..."
    docker build -t $IMAGE_NAME .
    log_ok "镜像构建完成"
}

# 运行容器
run_container() {
    log_info "启动容器..."
    docker run -d \
        --name $CONTAINER_NAME \
        --gpus all \
        -p $PORT:8080 \
        -v /var/run/docker.sock:/var/run/docker.sock \
        $IMAGE_NAME

    log_ok "容器已启动"
    log_info "等待服务就绪..."
    sleep 5
}

# 测试服务
test_service() {
    log_info "测试服务..."

    # 测试1: 检查容器运行
    if docker ps | grep -q $CONTAINER_NAME; then
        log_ok "容器运行中"
    else
        log_error "容器未运行"
        return 1
    fi

    # 测试2: 检查端口
    if curl -s -f http://localhost:$PORT/api/gpus > /dev/null 2>&1; then
        log_ok "服务响应正常"
    else
        log_error "服务无响应"
        return 1
    fi

    # 测试3: GPU列表
    log_info "检查GPU..."
    curl -s http://localhost:$PORT/api/gpus | python3 -m json.tool 2>/dev/null || \
        curl -s http://localhost:$PORT/api/gpus

    # 测试4: 提交测试任务
    log_info "提交测试任务..."
    RESP=$(curl -s -X POST http://localhost:$PORT/api/tasks \
        -H "Content-Type: application/json" \
        -d '{
            "name": "docker-test",
            "command": "echo hello from docker",
            "image": "ubuntu:latest",
            "gpu_required": 1,
            "priority": 5
        }')
    echo "$RESP" | python3 -m json.tool 2>/dev/null || echo "$RESP"

    log_ok "Docker 部署测试完成"
}

# 主程序
case "${1:-run}" in
    build)
        build_image
        ;;
    run)
        cleanup
        build_image
        run_container
        test_service
        ;;
    test)
        test_service
        ;;
    clean)
        cleanup
        ;;
    *)
        echo "用法: $0 [build|run|test|clean]"
        echo ""
        echo "  build - 构建镜像"
        echo "  run   - 构建并运行测试"
        echo "  test  - 测试运行中的服务"
        echo "  clean - 清理容器"
        ;;
esac
