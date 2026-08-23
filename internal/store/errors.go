package store

import (
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"task185-nvwear/internal/model"
)

// ErrConcurrentCreate 表示并发创建命中已有记录（非错误）：调用方应改读已存在实体并收敛。
// 用于 plan 创建等「相同逻辑键由并发请求先写入」的场景，避免把唯一约束当作数据库错误暴露。
var ErrConcurrentCreate = errors.New("concurrent create: entity already exists")

// IsConcurrentCreate 判定错误是否为并发收敛哨兵（容忍 %w 包装）。
func IsConcurrentCreate(err error) bool { return errors.Is(err, ErrConcurrentCreate) }

// mapSQLiteError 将 modernc/sqlite 驱动错误映射为领域错误。
// 唯一约束冲突（SQLITE_CONSTRAINT_UNIQUE）→ model.ErrConflict，
// 避免裸数据库约束错误泄漏到 HTTP 层。
func mapSQLiteError(err error) error {
	if err == nil {
		return nil
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%w: %s", model.ErrConflict, err.Error())
		}
	}
	return nil
}
