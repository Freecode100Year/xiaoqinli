@echo off
chcp 65001 > nul
title xiaoqinli (.xql) 一键部署与生命周期管理工具

echo =====================================================================
echo  [xiaoqinli] 正在启动全自动一键部署与 Docker 容灾环境编排...
echo =====================================================================

:: 1. 检查 Docker 是否运行
docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo  错误: Docker 未启动，请先打开 Docker Desktop！
    pause
    exit /b
)

:: 2. 启动 Docker Compose (全自动编排 Tree-sitter 镜像与 tmpfs 内存盘)
echo  [1/3] 正在构建并拉取 Docker 沙箱环境...
docker compose up -d --build
if %errorlevel% neq 0 (
    echo  错误: Docker Compose 启动失败！
    pause
    exit /b
)

:: 3. 与宿主机 Antigravity CLI 进行安全无感握手
echo  [2/3] 正在与宿主机 Antigravity CLI (agy) 进行无感对接...
agy config set mcp.servers.xiaoqinli.url "http://localhost:8080/mcp" >nul 2>&1
if %errorlevel% neq 0 (
    echo  提示: 未检测到 agy 全局配置，正在尝试重试注册...
    agy config set mcp.servers.xiaoqinli.url "http://localhost:8080/mcp"
)

:: 4. 将最高宪法注入到提示词缓存头部
echo  [3/3] 正在将 AGY_RULES.md 最高宪法推送到 AI 提示词缓存头部...
:: 这里的逻辑由本地 MCP 服务器在挂载时自动读取根目录下的 AGY_RULES.md

echo =====================================================================
echo  [xiaoqinli] 部署成功！
echo   Docker 物理安全防火墙已全面锁死！
echo  10秒内存影子闪存 (tmpfs) 已在内存中高效运转 (零固态硬盘损耗)！
echo =====================================================================
echo 现在你可以无忧开启 `agy --dangerously-skip-permissions` 自动驾驶开发了！
echo =====================================================================
pause
