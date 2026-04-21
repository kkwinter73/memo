// HTTPリクエストを受けて MemoStore のメソッドを呼ぶ。トランスポート層。

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// MemoHandler はハンドラが必要とする依存を束ねる
type MemoHandler struct {
	store *MemoStore
}

// NewMemoHandler は MemoHandler を生成する
func NewMemoHandler(store *MemoStore) *MemoHandler {
	return &MemoHandler{store: store}
}

// createMemoRequest は POST /memos のリクエストボディ
type createMemoRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// updateMemoRequest は PUT /memos/{id} のリクエストボディ
type updateMemoRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// errorResponse はエラー応答のJSON形式
type errorResponse struct {
	Error string `json:"error"`
}

// Create は POST /memos
func (h *MemoHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createMemoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	memo := h.store.Create(req.Title, req.Content)
	writeJSON(w, http.StatusCreated, memo)
}

// List は GET /memos
func (h *MemoHandler) List(w http.ResponseWriter, r *http.Request) {
	memos := h.store.GetAll()
	writeJSON(w, http.StatusOK, memos)
}

// Get は GET /memos/{id}
func (h *MemoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	memo, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "memo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, memo)
}

// Update は PUT /memos/{id}
func (h *MemoHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req updateMemoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	memo, err := h.store.Update(id, req.Title, req.Content)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "memo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, memo)
}

// Delete は DELETE /memos/{id}
func (h *MemoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "memo not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseID は URLパスから {id} を取り出して int に変換する
func parseID(r *http.Request) (int, error) {
	idStr := r.PathValue("id")
	return strconv.Atoi(idStr)
}

// writeJSON は任意の値を JSON にして書き出す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// ここは既にWriteHeader済みなので、ログ出す以外にできることがない
		// 本格的な実装ではロガーに流すけど今は省略
		return
	}
}

// writeError はエラー応答を書き出す
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
