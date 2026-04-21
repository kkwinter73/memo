package main

import (
	"log"
	"net/http"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // 時計スタート

		rec := &statusRecorder{ResponseWriter: w, status: 200} // ミラー板カバー装着
		next.ServeHTTP(rec, r)                                 // 店員を呼ぶ(カバー付きレジで)

		log.Printf("%s %s → %d (%v)", // 記録を読んでログに残す
			r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
