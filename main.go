package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/gorilla/mux"
)

var csvFile *os.File
var writer *csv.Writer

func getRealIP(r *http.Request) string {
	cfConnectingIP := r.Header.Get("CF-Connecting-IP")
	realIP := r.Header.Get("X-Real-IP")
	forwardedFor := r.Header.Get("X-Forwarded-For")
	remoteAddr := strings.Split(r.RemoteAddr, ":")[0]

	return cfConnectingIP + "," + realIP + "," + forwardedFor + "," + remoteAddr
}

func main() {
	file, err := os.Create("history.csv")
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer file.Close()

	csvFile = file
	writer = csv.NewWriter(csvFile)

	stat, _ := csvFile.Stat()
	if stat.Size() == 0 {
		writer.Write([]string{"IP", "计算表达式", "结果"})
		writer.Flush()
	}

	router := mux.NewRouter()
	router.HandleFunc("/", CalculateHandler).Methods("GET", "POST")

	log.Fatal(http.ListenAndServe(":12345", router))
}

func CalculateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	expression := r.Form.Get("expression")

	result, err := Calculate(expression)
	if err != nil {
		fmt.Fprintf(w, "计算失败")
		writeToCSV(getRealIP(r), expression, "计算失败")
	} else {
		fmt.Fprintf(w, "%v", result)
		writeToCSV(getRealIP(r), expression, fmt.Sprintf("%v", result))
	}
}

func Calculate(expression string) (interface{}, error) {
	eval, _ := govaluate.NewEvaluableExpression(expression)
	done := make(chan interface{}, 1)
	errs := make(chan error, 1)

	go func() {
		result, err := eval.Evaluate(nil)
		if err != nil {
			errs <- err
		} else {
			done <- result
		}
	}()

	select {
	case res := <-done:
		return res, nil
	case <-errs:
		return nil, fmt.Errorf("计算失败")
	case <-time.After(1 * time.Second):
		return nil, fmt.Errorf("计算失败")
	}
}

func writeToCSV(ip, expression, result string) {
	writer.Write([]string{ip, expression, result})
	writer.Flush()
}
