package main

import (
	"log"
	"net/http"
)

func main() {
	// 依存の組み立て
	store := NewMemoStore()
	h := NewMemoHandler(store)

	// ルーティング
	mux := http.NewServeMux()
	mux.HandleFunc("POST /memos", h.Create)
	mux.HandleFunc("GET /memos", h.List)
	mux.HandleFunc("GET /memos/{id}", h.Get)
	mux.HandleFunc("PUT /memos/{id}", h.Update)
	mux.HandleFunc("DELETE /memos/{id}", h.Delete)

	// サーバー起動
	addr := ":8080"
	log.Printf("listening on %s", addr)
	// if err := http.ListenAndServe(addr, mux); err != nil {
	// 	log.Fatal(err)
	// }

	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}
