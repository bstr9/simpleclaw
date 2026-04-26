// Package common 提供通用工具函数
package common

import (
	"runtime/debug"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// GoSafe 启动带有 panic 恢复的 goroutine
// 所有生产环境的 goroutine 都应使用此函数启动，防止 panic 导致进程崩溃
func GoSafe(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[GoSafe] goroutine panic 已恢复",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())))
			}
		}()
		fn()
	}()
}

// GoSafeWithErr 启动带有 panic 恢复的 goroutine，支持错误回调
// 当 goroutine 正常返回错误或发生 panic 时，都会调用 errHandler
func GoSafeWithErr(fn func() error, errHandler func(err error)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[GoSafe] goroutine panic 已恢复",
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())))
			}
		}()
		if err := fn(); err != nil {
			errHandler(err)
		}
	}()
}
