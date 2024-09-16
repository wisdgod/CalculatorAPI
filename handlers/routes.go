package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

func SetupRoutes(router *mux.Router) {
	router.HandleFunc("/", CalculateHandler).Methods("GET", "POST")
	router.HandleFunc("/calculator", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	}).Methods("GET")
	router.HandleFunc("/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "leaderboard.html")
	}).Methods("GET")
	router.HandleFunc("/leaderboard-data", LeaderboardDataHandler).Methods("GET")
}
