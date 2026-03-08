@echo off
REM GPU Scheduler 上传脚本 (Windows)
REM 使用方法: 双击运行此脚本

set REPO_NAME=waste-to-ai
set GITHUB_USER=day8reak

echo ========================================
echo   开始上传到 GitHub
echo ========================================

REM 检查 gh 是否安装
where gh >nul 2>nul
if %errorlevel% neq 0 (
    echo 正在安装 gh CLI...
    winget install GitHub.cli --accept-package-agreements --accept-source-agreements
)

REM 登录 GitHub
echo 请确保已登录 GitHub:
gh auth login

REM 创建仓库
echo 创建仓库 %GITHUB_USER%/%REPO_NAME%...
gh repo create %REPO_NAME% --public --source=. --description "废卡变AI - 利用闲置GPU/NPU运行推理服务" --push

echo.
echo ========================================
echo   上传完成!
echo ========================================
echo 仓库地址: https://github.com/%GITHUB_USER%/%REPO_NAME%
pause
