package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"io"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func InitDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "DLite.db")
	if err != nil {
		return nil, err
	}

	createDLiteTable := `
	CREATE TABLE IF NOT EXISTS DLite (
		token TEXT PRIMARY KEY,
		path TEXT,
		expire DATETIME
	);`

	createChecksumTable := `
	CREATE TABLE IF NOT EXISTS checksum (
		path TEXT PRIMARY KEY,
		hash TEXT
	);`

	if _, err := db.Exec(createDLiteTable); err != nil {
		return nil, err
	}
	if _, err := db.Exec(createChecksumTable); err != nil {
		return nil, err
	}
	go func() {
		start := time.Now()

		// 1. 清空 checksum 表
		if _, err := db.Exec("DELETE FROM checksum"); err != nil {
			log.Println("清空 checksum 表失败:", err)
			return
		}

		// 2. 并发读取 songs 目录并计算 md5
		fileMap := make(map[string]string) // path -> md5
		var mu sync.Mutex
		var wg sync.WaitGroup
		fileCount := 0

		err := filepath.Walk("songs", func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil // 忽略错误或文件夹
			}

			wg.Add(1)
			go func(fullPath string) {
				defer wg.Done()

				// 相对路径：songs/abc.mp3 -> abc.mp3
				relPath, _ := filepath.Rel("songs", fullPath)

				// 计算 md5
				hash, err := calcMD5(fullPath)
				if err != nil {
					log.Println("计算 MD5 失败:", fullPath, err)
					return
				}

				// 写入 map
				mu.Lock()
				fileMap[relPath] = hash
				fileCount++
				mu.Unlock()
			}(path)

			return nil
		})

		wg.Wait()
		if err != nil {
			log.Println("遍历 songs 目录失败:", err)
			return
		}

		// 3. 批量写入数据库
		tx, err := db.Begin()
		if err != nil {
			log.Println("开启事务失败:", err)
			return
		}
		stmt, err := tx.Prepare("INSERT INTO checksum (path, hash) VALUES (?, ?)")
		if err != nil {
			log.Println("准备语句失败:", err)
			return
		}
		defer func(stmt *sql.Stmt) {
			err := stmt.Close()
			if err != nil {

			}
		}(stmt)

		for path, hash := range fileMap {
			if _, err := stmt.Exec(path, hash); err != nil {
				log.Println("插入失败:", path, err)
			}
		}
		if err := tx.Commit(); err != nil {
			log.Println("提交事务失败:", err)
			return
		}

		// 4. 打印日志
		duration := time.Since(start)
		log.Printf("共处理 %d 个文件，用时 %.2fs\n", fileCount, duration.Seconds())
	}()

	return db, nil
}

// 计算文件的 MD5
func calcMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	hasher := md5.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func InsertToken(db *sql.DB, token, path string, expire time.Time) error {
	_, err := db.Exec("INSERT INTO DLite (token, path, expire) VALUES (?, ?, ?)", token, path, expire)
	return err
}

func GetTokenPath(db *sql.DB, token string) (string, error) {
	var path string
	err := db.QueryRow("SELECT path FROM DLite WHERE token = ?", token).Scan(&path)
	return path, err
}

func DeleteToken(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM DLite WHERE token = ?", token)
	return err
}

func DeleteExpiredTokens(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM DLite WHERE expire < ?", time.Now())
	return err
}
