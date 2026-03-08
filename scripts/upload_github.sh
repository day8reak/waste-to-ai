#!/bin/bash
# GPU Scheduler 上传脚本
# 使用方法: 运行此脚本完成 GitHub 上传

set -e

REPO_NAME="waste-to-ai"
GITHUB_USER="day8reak"

echo "========================================"
echo "  开始上传到 GitHub"
echo "========================================"

# 1. 安装 gh CLI (如果未安装)
if ! command -v gh &> /dev/null; then
    echo "正在安装 gh CLI..."
    if command -v winget &> /dev/null; then
        winget install GitHub.cli
    elif command -v choco &> /dev/null; then
        choco install gh
    else
        echo "请手动安装 gh CLI: https://github.com/cli/cli#installation"
        exit 1
    fi
fi

# 2. 登录 GitHub
echo "请确保已登录 GitHub:"
gh auth login

# 3. 创建仓库
echo "创建仓库 $GITHUB_USER/$REPO_NAME..."
gh repo create $REPO_NAME --public --source=. --description "废卡变AI - 利用闲置GPU/NPU运行推理服务" --push

echo ""
echo "========================================"
echo "  上传完成!"
echo "========================================"
echo "仓库地址: https://github.com/$GITHUB_USER/$REPO_NAME"
