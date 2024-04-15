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
	router.Use(logRequestMiddleware)
	router.HandleFunc("/", CalculateHandler).Methods("GET", "POST")
	router.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=history.csv")
		http.ServeFile(w, r, "history.csv")
	}).Methods("GET")

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
	eval, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		usage := "请提供一个合法的表达式。你可以使用的操作符包括 +, -, *, / 等。例如: '5+3' 或 '2*8'。" +
			"你可以使用curl命令尝试一下:" +
			"\ncurl -G -d 'expression=5+3' http://ip:12345/" +
			"\ncurl -X POST -d 'expression=2*8' http://ip:12345/" +
			"\n你也可以在浏览器中测试，只需要在浏览器的地址栏输入：http://ip:12345/?expression=5+3"
		return nil, fmt.Errorf("创建可求值表达式失败: %v. %s", err, usage)
	}
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
