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
