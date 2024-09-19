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
