package main

import (
	"CalculatorAPI/calc"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func getRealIP(r *http.Request) string {
	cfConnectingIP := r.Header.Get("CF-Connecting-IP")
	realIP := r.Header.Get("X-Real-IP")
	forwardedFor := r.Header.Get("X-Forwarded-For")
	remoteAddr := strings.Split(r.RemoteAddr, ":")[0]

	return cfConnectingIP + "," + realIP + "," + forwardedFor + "," + remoteAddr
}

func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			fmt.Printf("Error loading time location: %v\n", err)
			loc = time.UTC
		}
		startInCST := start.In(loc)

		realIP := getRealIP(r)

		ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		fmt.Printf("%s - - [%s] \"%s %s %s\" %d %d\n",
			realIP,
			startInCST.Format("02/Jan/2006 15:04:05"),
			r.Method,
			r.RequestURI,
			r.Proto,
			ww.statusCode,
			duration.Milliseconds(),
		)
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTables()

	router := mux.NewRouter()
	router.Use(logRequestMiddleware)
	router.HandleFunc("/", CalculateHandler).Methods("GET", "POST")
	router.HandleFunc("/calculator", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	}).Methods("GET")
	router.HandleFunc("/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "leaderboard.html")
	}).Methods("GET")
	router.HandleFunc("/leaderboard-data", LeaderboardDataHandler).Methods("GET")

	log.Fatal(http.ListenAndServe(":12345", router))
}

func LeaderboardDataHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT ip, total_value, count, min_value, min_expression, max_value, max_expression FROM leaderboard ORDER BY total_value DESC LIMIT 1000")
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
		entry.IP = getRandomNonEmptyIP(entry.IP)
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

func getRandomNonEmptyIP(ip string) string {
	ips := strings.Split(ip, ",")
	nonEmptyIPs := []string{}
	for _, ip := range ips {
		if strings.TrimSpace(ip) != "" {
			nonEmptyIPs = append(nonEmptyIPs, ip)
		}
	}

	if len(nonEmptyIPs) == 0 {
		return "unknown"
	}

	return nonEmptyIPs[rand.Intn(len(nonEmptyIPs))]
}

func createTables() {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ip TEXT,
			expression TEXT,
			result TEXT
		);
		CREATE TABLE IF NOT EXISTS leaderboard (
			ip TEXT PRIMARY KEY,
			total_value REAL,
			count INTEGER,
			min_value REAL,
			min_expression TEXT,
			max_value REAL,
			max_expression TEXT
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func CalculateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	expression := r.Form.Get("expression")
	ip := getRealIP(r)

	result, err := Calculate(expression, ip)
	response := map[string]interface{}{}

	if err != nil {
		response["error"] = fmt.Sprintf("计算失败：%v", err)
		writeToDB(ip, expression, "计算失败")
	} else {
		response["value"] = result
		writeToDB(ip, expression, result.(string))

		if strings.HasPrefix(expression, "rand(") {
			response["showLeaderboard"] = true
		} else {
			response["showLeaderboard"] = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func Calculate(expression string, ip string) (interface{}, error) {
	result, err := calc.Calculate(expression)
	if err != nil {
		return nil, fmt.Errorf("计算失败: %v", err)
	}

	// 处理 rand() 函数的特殊情况
	if strings.HasPrefix(expression, "rand(") {
		// result 已经是 *big.Float 类型，不需要转换
		updateLeaderboard(ip, result, expression)
	}

	return result.Text('f', 10), nil // 返回字符串形式的结果，保留10位小数
}

func writeToDB(ip, expression, result string) {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := writeToDBOnce(ip, expression, result)
		if err != nil {
			if isLockedError(err) {
				time.Sleep(retryDelay)
				continue
			}
			log.Println("Error writing to DB:", err)
			return
		}
		return
	}

	log.Println("Max retries reached for writing to DB")
}

func writeToDBOnce(ip, expression, result string) error {
	_, err := db.Exec("INSERT INTO history (ip, expression, result) VALUES (?, ?, ?)", ip, expression, result)
	if err != nil {
		return err
	}
	return nil
}

func updateLeaderboard(ip string, value *big.Float, expression string) {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := updateLeaderboardOnce(ip, value, expression)
		if err != nil {
			if isLockedError(err) {
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
	tx, err := db.Begin()
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

func isLockedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./calculator.db")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA journal_mode = WAL;")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	return db, nil
}
