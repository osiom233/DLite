package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var db *sql.DB

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
	MD5     string `json:"md5,omitempty"`
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[请求] %s %s?%s 来自 %s", r.Method, r.URL.Path, r.URL.RawQuery, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func main() {
	var err error
	db, err = InitDB()
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/request", handleRequest)
	mux.HandleFunc("/download", handleDownload)
	mux.HandleFunc("/direct", handleDirect)
	mux.HandleFunc("/md5", handleMD5)

	log.Println("服务启动在 :8080")
	if err := http.ListenAndServe(":8080", logMiddleware(mux)); err != nil {
		log.Fatal("服务启动失败:", err)
	}
}

// 申请 token
func handleRequest(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	path, err := PathHelper(fileParam)
	if err != nil {
		respondJSON(w, Response{Success: false, Message: err.Error()})
		return
	}
	token := time.Now().Format("20060102150405") + "_" + RandString(6)
	expire := time.Now().Add(5 * time.Minute)

	if err := InsertToken(db, token, path, expire); err != nil {
		respondJSON(w, Response{Success: false, Message: "写入数据库失败"})
		return
	}

	respondJSON(w, Response{Success: true, Token: token})
}

// 下载文件
func handleDownload(w http.ResponseWriter, r *http.Request) {
	err := DeleteExpiredTokens(db)
	if err != nil {
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		respondJSON(w, Response{Success: false, Message: "参数 token 缺失"})
		return
	}

	path, err := GetTokenPath(db, token)
	if err != nil {
		respondJSON(w, Response{Success: false, Message: "token 无效"})
		return
	}
	_ = DeleteToken(db, token)

	sendFile(w, path)
}

// 直链下载
func handleDirect(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		respondJSON(w, Response{Success: false, Message: "参数 file 缺失"})
		return
	}
	path := "songs/" + fileParam
	sendFile(w, path)
}

// 获取 MD5
func handleMD5(w http.ResponseWriter, r *http.Request) {
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		respondJSON(w, Response{Success: false, Message: "参数 file 缺失"})
		return
	}
	path := "songs/" + fileParam

	data, err := os.ReadFile(path)
	if err != nil {
		respondJSON(w, Response{Success: false, Message: "读取文件失败"})
		return
	}

	sum := md5Sum(data)
	respondJSON(w, Response{Success: true, MD5: sum})
}

func md5Sum(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func getFileName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func sendFile(w http.ResponseWriter, path string) {
	file, err := os.Open(path)
	if err != nil {
		respondJSON(w, Response{Success: false, Message: "找不到文件"})
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	w.Header().Set("Content-Disposition", "attachment; filename="+getFileName(path))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, file)
}

func respondJSON(w http.ResponseWriter, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// RandString 生成随机字符串（用于 token）
func RandString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // 避免重复
	}
	return string(b)
}

func PathHelper(path string) (string, error) {
	if path == "" || strings.Contains(path, "..") {
		return "", errors.New("illegal request")
	}
	return path, nil
}
