package main

import (
	"fmt"
	"log"
	"net/http"
	"server-content/internal/config"
	"server-content/internal/db/database"
	"server-content/internal/handlers"
	"server-content/internal/logger"

	"github.com/joho/godotenv"
)

func main() {
	log.Println("🚀 Starting Web Content Server")

	// Load .env (optional)
	_ = godotenv.Load()

	// Load config
	config.Load()

	// Init file logger (writes to stdout + rotating log file)
	logCloser, err := logger.Init(config.AppConfig.LogPath)
	if err != nil {
		log.Printf("⚠️ File logging disabled: %v", err)
	} else {
		defer logCloser.Close()
		log.Printf("📝 Logging to: %s (max 25MB per file)", config.AppConfig.LogPath)
	}

	// Connect to MongoDB
	if err := database.Connect(); err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}
	defer database.Disconnect()
	log.Println("✅ MongoDB connected")

	// Get port from environment or use default
	port := config.AppConfig.Port
	if port == "" {
		port = "8082"
	}

	// Initialize handlers
	h := handlers.NewHandler(handlers.Handler{})

	// Routes
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"server-content"}`)
	})
	http.HandleFunc("/logs", h.HandleLogList)
	http.HandleFunc("/logs/", h.HandleLogFile)
	http.HandleFunc("/", h.Home)

	fmt.Printf("Server started at http://localhost:%s\n", port)

	// CORS middleware
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(":"+port, corsHandler); err != nil {
		log.Println("Error starting server:", err)
	}
}
