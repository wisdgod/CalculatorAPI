package db

import (
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"time"
)

func UpdateLeaderboard(ip string, value *big.Float, expression string) {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := updateLeaderboardOnce(ip, value, expression)
		if err != nil {
			if IsLockedError(err) {
				time.Sleep(retryDelay)
				continue
			}
			log.Println("Error updating leaderboard:", err)
			return
		}
		return
	}

	log.Println("Max retries reached for updating leaderboard")
}

func updateLeaderboardOnce(ip string, value *big.Float, expression string) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var totalValue float64
	var count int
	var minValue, maxValue float64
	var minExpression, maxExpression string

	err = tx.QueryRow("SELECT total_value, count, min_value, min_expression, max_value, max_expression FROM leaderboard WHERE ip = ?", ip).Scan(&totalValue, &count, &minValue, &minExpression, &maxValue, &maxExpression)

	valueFloat, _ := value.Float64()

	if err == sql.ErrNoRows {
		_, err = tx.Exec("INSERT INTO leaderboard (ip, total_value, count, min_value, min_expression, max_value, max_expression) VALUES (?, ?, 1, ?, ?, ?, ?)", ip, valueFloat, valueFloat, expression, valueFloat, expression)
	} else if err == nil {
		totalValue += valueFloat
		count++
		if valueFloat < minValue {
			minValue = valueFloat
			minExpression = expression
		}
		if valueFloat > maxValue {
			maxValue = valueFloat
			maxExpression = expression
		}
		_, err = tx.Exec("UPDATE leaderboard SET total_value = ?, count = ?, min_value = ?, min_expression = ?, max_value = ?, max_expression = ? WHERE ip = ?", totalValue, count, minValue, minExpression, maxValue, maxExpression, ip)
	}

	if err != nil {
		return fmt.Errorf("error updating leaderboard: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}
