@echo off
REM Change to the directory where the script is located
cd /d "%~dp0"

REM Check if the dist directory exists, if not, create it
if not exist "%~dp0dist" (
    mkdir "%~dp0dist"
)

REM Build for Linux amd64
set GOOS=linux
set GOARCH=amd64
go build -a -o "%~dp0dist\CalculatorAPI-linux-amd64" main.go

REM Build for Windows amd64
set GOOS=windows
set GOARCH=amd64
go build -a -o "%~dp0dist\CalculatorAPI-windows-amd64.exe" main.go

REM Reset environment variables
set GOOS=
set GOARCH=
