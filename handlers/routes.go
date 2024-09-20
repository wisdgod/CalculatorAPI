package handlers

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

//go:embed calculator.html leaderboard.html help.html
var content embed.FS

var templates = template.Must(template.ParseFS(content, "*.html"))

func SetupRoutes(router *mux.Router) {
	router.HandleFunc("/", CalculateHandler).Methods("GET", "POST")
	router.HandleFunc("/calculator", ServeCalculatorPage).Methods("GET")
	router.HandleFunc("/leaderboard", ServeLeaderboardPage).Methods("GET")
	router.HandleFunc("/help", ServeHelpPage).Methods("GET")
	router.HandleFunc("/leaderboard-data", LeaderboardDataHandler).Methods("GET")
}

func ServeCalculatorPage(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "calculator.html", nil)
}

func ServeLeaderboardPage(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "leaderboard.html", nil)
}

func ServeHelpPage(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "help.html", nil)
}
