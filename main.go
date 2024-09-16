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
	flag.StringVar(&port, "port", "12345", "Port to run the server on")
	flag.StringVar(&dbDir, "dbDir", ".", "Directory for the SQLite database")
	flag.StringVar(&logDir, "logDir", "./logs", "Directory for log files")
}

func main() {
	flag.Parse()

	// Initialize logging
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}
	logFile := filepath.Join(logDir, time.Now().Format("20060102_1504")+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	// Create a multi-writer to write to both file and console
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)

	// Initialize database
	dbPath := filepath.Join(dbDir, "calculator.db")
	db.DB, err = db.InitDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.DB.Close()

	db.CreateTables()

	// Setup router
	router := mux.NewRouter()
	router.Use(middleware.LogRequestMiddleware)
	handlers.SetupRoutes(router)

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
