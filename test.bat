@echo off
REM 这是一个用于测试和运行Go应用程序的批处理文件

cd /d "%~dp0"

set CGO_ENABLED=1

REM 运行Go测试
echo 正在运行测试...
go run .

set CGO_ENABLED=

REM 暂停以保持命令提示符窗口打开
REM pause

REM 如何清除Go模块缓存：
REM 要清除Go模块缓存，可以使用以下命令：
REM go clean -modcache
REM go clean -cache -modcache -i -r