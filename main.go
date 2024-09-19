package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"CalculatorAPI/db"
	"CalculatorAPI/handlers"
	"CalculatorAPI/middleware"

	"github.com/gorilla/mux"
)

var (
	port   string
	dbDir  string
	logDir string
)

func init() {
	// 定义命令行标志
	flag.StringVar(&port, "port", "12345", "运行服务器的端口")
	flag.StringVar(&dbDir, "dbDir", ".", "SQLite数据库的目录")
	flag.StringVar(&logDir, "logDir", "./logs", "日志文件的目录")
}

func main() {
	flag.Parse()

	// 初始化日志
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}
	logFile := filepath.Join(logDir, time.Now().Format("20060102_1504")+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}
	defer f.Close()

	// 创建一个多写入器，写入文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	// 初始化数据库
	dbPath := filepath.Join(dbDir, "calculator.db")
	db.DB, err = db.InitDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.DB.Close()

	db.CreateTables()

	// 设置路由器
	router := mux.NewRouter()
	router.Use(middleware.LogRequestMiddleware)
	handlers.SetupRoutes(router)

	// 启动服务器并实现优雅关闭
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// 等待中断信号以优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务器强制关闭: %v", err)
	}

	log.Println("服务器已退出")
}
