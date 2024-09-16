package handlers

import (
	"encoding/json"
	"math/big"
	"net/http"

	"CalculatorAPI/db"
	"CalculatorAPI/utils"
)

func LeaderboardDataHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT ip, total_value, count, min_value, min_expression, max_value, max_expression FROM leaderboard ORDER BY total_value DESC LIMIT 1000")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type LeaderboardEntry struct {
		IP            string `json:"ip"`
		TotalValue    string `json:"total_value"`
		Count         int    `json:"count"`
		MinValue      string `json:"min_value"`
		MinExpression string `json:"min_expression"`
		MaxValue      string `json:"max_value"`
		MaxExpression string `json:"max_expression"`
	}

	var leaderboard []LeaderboardEntry

	for rows.Next() {
		var entry LeaderboardEntry
		var totalValue, minValue, maxValue float64
		err := rows.Scan(&entry.IP, &totalValue, &entry.Count, &minValue, &entry.MinExpression, &maxValue, &entry.MaxExpression)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		entry.TotalValue = new(big.Float).SetFloat64(totalValue).String()
		entry.MinValue = new(big.Float).SetFloat64(minValue).String()
		entry.MaxValue = new(big.Float).SetFloat64(maxValue).String()
		entry.IP = utils.GetRandomNonEmptyIP(entry.IP)
		leaderboard = append(leaderboard, entry)
	}

	for len(leaderboard) < 1000 {
		leaderboard = append(leaderboard, LeaderboardEntry{
			IP:            "unknown",
			TotalValue:    "0",
			Count:         0,
			MinValue:      "0",
			MinExpression: "null",
			MaxValue:      "0",
			MaxExpression: "null",
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}
