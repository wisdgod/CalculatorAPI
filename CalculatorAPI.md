calc/calc.go
```
package calc

/*
#cgo LDFLAGS: -L. -lcalc_parse -Wl,-rpath,$ORIGIN
#include "calculator.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"math/big"
	"unsafe"
)

const (
	initialBufferSize = 1024 * 64        // 64KB
	maxBufferSize     = 1024 * 1024 * 64 // 64MB
	errorBufferSize   = 1024             // 1KB for error messages
)

// Calculate 评估一个数学表达式并返回 big.Float 类型的结果。
func Calculate(expr string) (*big.Float, error) {
	// 创建一个新的计算器实例
	calc := C.create_calculator()
	if calc == nil {
		return nil, errors.New("创建计算器实例失败")
	}
	defer C.destroy_calculator(calc)

	// 将 Go 字符串转换为 C 字符串
	cExpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cExpr))

	// 使用动态缓冲区大小
	bufSize := initialBufferSize
	var result string
	var resultCode C.CalcErrorCode

	// 创建错误消息缓冲区
	errorBuf := make([]byte, errorBufferSize)
	cErrorBuf := (*C.char)(unsafe.Pointer(&errorBuf[0]))

	for bufSize <= maxBufferSize {
		buf := make([]byte, bufSize)
		cBuf := (*C.char)(unsafe.Pointer(&buf[0]))

		resultCode = C.calculate_expression(calc, cExpr, cBuf, C.size_t(bufSize), cErrorBuf, C.size_t(errorBufferSize))

		if resultCode == C.CALC_SUCCESS {
			result = C.GoString(cBuf)
			break
		} else if resultCode != C.CALC_ERROR_BUFFER_TOO_SMALL {
			return nil, fmt.Errorf("计算失败: %s", C.GoString(cErrorBuf))
		}

		// 增加缓冲区大小并重试
		bufSize *= 2
	}

	if resultCode != C.CALC_SUCCESS {
		return nil, errors.New("结果超出最大缓冲区大小")
	}

	// 将结果字符串解析为 big.Float
	f := new(big.Float)
	_, _, err := f.Parse(result, 10)
	if err != nil {
		return nil, fmt.Errorf("解析结果失败: %v", err)
	}

	return f, nil
}
```
db/history.go
```
package db

import (
	"log"
	"time"
)

func WriteToDB(ip, expression, result string) {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := writeToDBOnce(ip, expression, result)
		if err != nil {
			if IsLockedError(err) {
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
	_, err := DB.Exec("INSERT INTO history (ip, expression, result) VALUES (?, ?, ?)", ip, expression, result)
	if err != nil {
		return err
	}
	return nil
}
```
db/init.go
```
package db

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
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

func CreateTables() {
	_, err := DB.Exec(`
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

func IsLockedError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}
```
db/leaderboard.go
```
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
```
handlers/calculate.go
```
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"CalculatorAPI/db"
	"CalculatorAPI/utils"

	"CalculatorAPI/calc"
)

func CalculateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	expression := r.Form.Get("expression")
	if expression == "" {
		http.Error(w, `{"error": "Expression cannot be empty"}`, http.StatusBadRequest)
		return
	}

	ip := utils.GetRealIP(r)

	result, err := Calculate(expression, ip)
	response := map[string]interface{}{}

	if err != nil {
		response["error"] = fmt.Sprintf("计算失败：%v", err)
		db.WriteToDB(ip, expression, "计算失败")
	} else {
		response["value"] = result
		db.WriteToDB(ip, expression, result.(string))

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

	if strings.HasPrefix(expression, "rand(") {
		db.UpdateLeaderboard(ip, result, expression)
	}

	// 将结果转换为字符串并去除尾部多余的零
	resultStr := result.Text('f', 999999)
	resultStr = strings.TrimRight(resultStr, "0")
	resultStr = strings.TrimSuffix(resultStr, ".")

	return resultStr, nil
}
```
handlers/leaderboard.go
```
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
```
handlers/routes.go
```
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
```
index.html
```
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Calculator | 计算器</title>
    <style>
        :root {
            --background-color: #f0f2f5;
            --text-color: #333333;
            --container-background-color: #ffffff;
            --border-color: #dddddd;
            --display-background-color: #f9f9f9;
            --button-background-color: #e0e0e0;
            --button-text-color: #333333;
            --button-hover-background-color: #d0d0d0;
            --button-active-background-color: #c0c0c0;
            --clear-button-background-color: #ffcccc;
            --clear-button-hover-background-color: #ffb3b3;
            --equal-button-background-color: #ccffcc;
            --equal-button-hover-background-color: #b3ffb3;
            --placeholder-color: #999999;
            --error-background-color: #ffe6e6;
            --error-text-color: #cc0000;
        }

        [data-theme="dark"] {
            --background-color: #121212;
            --text-color: #ffffff;
            --container-background-color: #1e1e1e;
            --border-color: #333333;
            --display-background-color: #2e2e2e;
            --button-background-color: #3a3a3a;
            --button-text-color: #ffffff;
            --button-hover-background-color: #4a4a4a;
            --button-active-background-color: #5a5a5a;
            --clear-button-background-color: #ff6666;
            --clear-button-hover-background-color: #ff4d4d;
            --equal-button-background-color: #66ff66;
            --equal-button-hover-background-color: #4dff4d;
            --placeholder-color: #aaaaaa;
            --error-background-color: #4d0000;
            --error-text-color: #ff6666;
        }

        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: var(--background-color);
            color: var(--text-color);
            margin: 0;
            padding: 0;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            transition: background-color 0.3s, color 0.3s;
        }
        .container {
            background: var(--container-background-color);
            padding: 20px;
            border-radius: 12px;
            box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
            max-width: 400px;
            width: 100%;
            box-sizing: border-box;
            display: flex;
            flex-direction: column;
            align-items: center;
            transition: background-color 0.3s;
        }
        h1 {
            text-align: center;
            margin-bottom: 20px;
        }
        .display {
            width: 100%;
            padding: 20px;
            border: 1px solid var(--border-color);
            border-radius: 8px;
            font-size: 24px;
            margin-bottom: 20px;
            background-color: var(--display-background-color);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            box-sizing: border-box;
            color: var(--text-color);
        }
        .display::placeholder {
            color: var(--placeholder-color);
        }
        .buttons {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 10px;
            margin-bottom: 20px;
        }
        button {
            padding: 20px;
            background-color: var(--button-background-color);
            border: none;
            border-radius: 8px;
            color: var(--button-text-color);
            font-size: 18px;
            cursor: pointer;
            transition: background-color 0.3s, color 0.3s;
        }
        button:hover {
            background-color: var(--button-hover-background-color);
        }
        button:active {
            background-color: var(--button-active-background-color);
        }
        button.clear {
            background-color: var(--clear-button-background-color);
        }
        button.clear:hover {
            background-color: var(--clear-button-hover-background-color);
        }
        button.equal {
            background-color: var(--equal-button-background-color);
            grid-column: span 4;
        }
        button.equal:hover {
            background-color: var(--equal-button-hover-background-color);
        }
        #result {
            margin-top: 20px;
            font-size: 18px;
            text-align: center;
            display: none;
        }
        .error-message {
            background-color: var(--error-background-color);
            color: var(--error-text-color);
            padding: 10px;
            border-radius: 8px;
            max-height: 100px;
            overflow-y: auto;
            width: 100%;
            box-sizing: border-box;
            margin-top: 20px;
            display: none;
        }
        .theme-toggle {
            position: absolute;
            top: 20px;
            right: 20px;
            background: none;
            border: none;
            font-size: 18px;
            cursor: pointer;
            color: var(--text-color);
        }
    </style>
    <script>
        function applyTheme(theme) {
            document.documentElement.setAttribute('data-theme', theme);
        }

        function toggleTheme() {
            const currentTheme = document.documentElement.getAttribute('data-theme');
            const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
            applyTheme(newTheme);
            localStorage.setItem('theme', newTheme);
        }

        document.addEventListener('DOMContentLoaded', () => {
            const savedTheme = localStorage.getItem('theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
            applyTheme(savedTheme);
            document.getElementById('theme-toggle').addEventListener('click', toggleTheme);
        });

        function appendValue(value) {
            const display = document.getElementById("expression");
            display.value += value;
        }

        function clearDisplay() {
            document.getElementById("expression").value = '';
            document.getElementById("result").innerText = '';
            document.getElementById("error").innerText = '';
            document.getElementById("result").style.display = 'none';
            document.getElementById("error").style.display = 'none';
            const leaderboardBtn = document.getElementById("leaderboard-btn");
            if (leaderboardBtn) {
                leaderboardBtn.remove();
            }
        }

        function wrapExpressionWithFunction(func) {
            const display = document.getElementById("expression");
            const currentValue = display.value || "0"; // Default to "0" if the input is empty
            display.value = `${func}(${currentValue})`;
        }

        // function validateExpression(expression) {
        //     try {
        //         // Check for balanced parentheses
        //         let stack = [];
        //         for (let char of expression) {
        //             if (char === '(') {
        //                 stack.push(char);
        //             } else if (char === ')') {
        //                 if (stack.length === 0) {
        //                     return false;
        //                 }
        //                 stack.pop();
        //             }
        //         }
        //         return stack.length === 0;
        //     } catch (e) {
        //         return false;
        //     }
        // }

        async function calculate() {
            const expression = document.getElementById("expression").value;

            // if (!validateExpression(expression)) {
            //     document.getElementById("error").innerText = "Invalid expression. Please check your input.";
            //     document.getElementById("error").style.display = 'block';
            //     document.getElementById("result").style.display = 'none';
            //     return;
            // }

            const response = await fetch("/", {
                method: "POST",
                headers: {
                    "Content-Type": "application/x-www-form-urlencoded"
                },
                body: `expression=${encodeURIComponent(expression)}`
            });
            const result = await response.json();
            if (response.ok) {
                document.getElementById("result").innerText = `Result | 结果: ${result.value}`;
                document.getElementById("result").style.display = 'block';
                document.getElementById("error").style.display = 'none';
                if (result.showLeaderboard) {
                    createLeaderboardButton();
                }
            } else {
                document.getElementById("error").innerText = result.error;
                document.getElementById("error").style.display = 'block';
                document.getElementById("result").style.display = 'none';
                const leaderboardBtn = document.getElementById("leaderboard-btn");
                if (leaderboardBtn) {
                    leaderboardBtn.remove();
                }
            }
        }

        function createLeaderboardButton() {
            const container = document.querySelector('.container');
            let leaderboardBtn = document.getElementById("leaderboard-btn");
            if (!leaderboardBtn) {
                leaderboardBtn = document.createElement("button");
                leaderboardBtn.id = "leaderboard-btn";
                leaderboardBtn.innerText = "View Leaderboard | 查看排行榜";
                leaderboardBtn.onclick = () => {
                    window.location.href = '/leaderboard';
                };
                leaderboardBtn.style.marginTop = "20px";
                leaderboardBtn.style.padding = "10px 20px";
                leaderboardBtn.style.backgroundColor = "var(--button-background-color)";
                leaderboardBtn.style.border = "none";
                leaderboardBtn.style.borderRadius = "8px";
                leaderboardBtn.style.color = "var(--button-text-color)";
                leaderboardBtn.style.fontSize = "18px";
                leaderboardBtn.style.cursor = "pointer";
                leaderboardBtn.style.transition = "background-color 0.3s, color 0.3s";
                leaderboardBtn.onmouseover = () => {
                    leaderboardBtn.style.backgroundColor = "var(--button-hover-background-color)";
                };
                leaderboardBtn.onmouseout = () => {
                    leaderboardBtn.style.backgroundColor = "var(--button-background-color)";
                };
                leaderboardBtn.onmousedown = () => {
                    leaderboardBtn.style.backgroundColor = "var(--button-active-background-color)";
                };
                container.appendChild(leaderboardBtn);
            }
        }
    </script>
</head>
<body>
    <div class="container">
        <h1>Calculator | 计算器</h1>
        <input type="text" id="expression" class="display" placeholder="Enter expression | 输入表达式">
        <div class="buttons">
            <button onclick="appendValue('7')">7</button>
            <button onclick="appendValue('8')">8</button>
            <button onclick="appendValue('9')">9</button>
            <button onclick="appendValue('/')">/</button>
            <button onclick="appendValue('4')">4</button>
            <button onclick="appendValue('5')">5</button>
            <button onclick="appendValue('6')">6</button>
            <button onclick="appendValue('*')">*</button>
            <button onclick="appendValue('1')">1</button>
            <button onclick="appendValue('2')">2</button>
            <button onclick="appendValue('3')">3</button>
            <button onclick="appendValue('-')">-</button>
            <button onclick="appendValue('0')">0</button>
            <button onclick="appendValue('.')">.</button>
            <button class="clear" onclick="clearDisplay()">C</button>
            <button onclick="appendValue('+')">+</button>
            <button class="equal" onclick="calculate()">=</button>
            <!-- Advanced functions -->
            <button onclick="wrapExpressionWithFunction('sqrt')">sqrt</button>
            <button onclick="wrapExpressionWithFunction('pow')">pow</button>
            <button onclick="wrapExpressionWithFunction('rand')">rand</button>
            <button onclick="wrapExpressionWithFunction('sin')">sin</button>
            <button onclick="wrapExpressionWithFunction('cos')">cos</button>
            <button onclick="wrapExpressionWithFunction('tan')">tan</button>
            <button onclick="wrapExpressionWithFunction('log')">log</button>
            <button onclick="wrapExpressionWithFunction('exp')">exp</button>
        </div>
        <p id="result"></p>
        <div id="error" class="error-message"></div>
    </div>
    <button id="theme-toggle" class="theme-toggle">Toggle Theme | 切换主题</button>
</body>
</html>
```
leaderboard.html
```
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Leaderboard | 排行榜</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: #f0f2f5;
            color: #333333;
            margin: 0;
            padding: 20px;
            display: flex;
            justify-content: center;
            align-items: center;
            flex-direction: column;
        }
        table {
            width: 100%;
            max-width: 800px;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th, td {
            padding: 10px;
            border: 1px solid #dddddd;
            text-align: left;
        }
        th {
            background-color: #f9f9f9;
        }
    </style>
</head>
<body>
    <h1>Leaderboard | 排行榜</h1>
    <table>
        <thead>
            <tr>
                <th>IP</th>
                <th>Total Value</th>
                <th>Count</th>
                <th>Min Value</th>
                <th>Min Expression</th>
                <th>Max Value</th>
                <th>Max Expression</th>
            </tr>
        </thead>
        <tbody id="leaderboard-body">
            <!-- Rows will be inserted here by JavaScript -->
        </tbody>
    </table>
    <script>
        async function fetchLeaderboard() {
            const response = await fetch('/leaderboard-data');
            const data = await response.json();
            const tbody = document.getElementById('leaderboard-body');
            tbody.innerHTML = '';

            data.forEach((row, index) => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${row.ip}</td>
                    <td>${row.total_value}</td>
                    <td>${row.count}</td>
                    <td>${row.min_value}</td>
                    <td>${row.min_expression}</td>
                    <td>${row.max_value}</td>
                    <td>${row.max_expression}</td>
                `;
                tbody.appendChild(tr);
            });
        }

        fetchLeaderboard();
    </script>
</body>
</html>
```
main.go
```
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
```
middleware/log_request.go
```
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"CalculatorAPI/utils"
)

func LogRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		loc, err := time.LoadLocation("Asia/Shanghai")
		if err != nil {
			fmt.Printf("Error loading time location: %v\n", err)
			loc = time.UTC
		}
		startInCST := start.In(loc)

		realIP := utils.GetRealIP(r)

		ww := &utils.ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		fmt.Printf("%s - - [%s] \"%s %s %s\" %d %d\n",
			realIP,
			startInCST.Format("02/Jan/2006 15:04:05"),
			r.Method,
			r.RequestURI,
			r.Proto,
			ww.StatusCode,
			duration.Milliseconds(),
		)
	})
}
```
utils/ip.go
```
package utils

import (
	"math/rand"
	"net/http"
	"strings"
)

func GetRealIP(r *http.Request) string {
	cfConnectingIP := r.Header.Get("CF-Connecting-IP")
	realIP := r.Header.Get("X-Real-IP")
	forwardedFor := r.Header.Get("X-Forwarded-For")
	remoteAddr := strings.Split(r.RemoteAddr, ":")[0]

	return cfConnectingIP + "," + realIP + "," + forwardedFor + "," + remoteAddr
}

func GetRandomNonEmptyIP(ip string) string {
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
```
utils/response_writer.go
```
package utils

import (
	"net/http"
)

type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
}

func (rw *ResponseWriterWrapper) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
```
