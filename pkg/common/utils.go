// Package common 提供通用工具函数
package common

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileSize 获取文件大小
// 支持 *os.File、[]byte、io.ReadSeeker 等类型
func FileSize(file any) (int64, error) {
	switch f := file.(type) {
	case *os.File:
		info, err := f.Stat()
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	case []byte:
		return int64(len(f)), nil
	case *bytes.Buffer:
		return int64(f.Len()), nil
	case *bytes.Reader:
		return int64(f.Len()), nil
	case io.ReadSeeker:
		pos, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		size, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		_, err = f.Seek(pos, io.SeekStart)
		return size, err
	default:
		return 0, ErrUnsupportedType
	}
}

// ErrUnsupportedType 不支持的类型错误
var ErrUnsupportedType = NewError("unsupported type")

// AppError 应用错误类型
type AppError struct {
	Message string
}

// NewError 创建新错误
func NewError(message string) *AppError {
	return &AppError{Message: message}
}

// Error 实现error接口
func (e *AppError) Error() string {
	return e.Message
}

// SplitStringByUTF8Length 按UTF-8字节长度分割字符串
// maxLength: 每段最大字节长度
// maxSplit: 最大分割次数，0表示不限制
func SplitStringByUTF8Length(s string, maxLength int, maxSplit int) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	bytes := []byte(s)
	start := 0

	for start < len(bytes) {
		if shouldAddRemaining(&result, maxSplit) {
			result = append(result, string(bytes[start:]))
			break
		}

		end := calculateEndPosition(bytes, start, maxLength)
		result = append(result, string(bytes[start:end]))
		start = end
	}

	return result
}

// shouldAddRemaining 检查是否应直接添加剩余内容
func shouldAddRemaining(result *[]string, maxSplit int) bool {
	return maxSplit > 0 && len(*result) >= maxSplit
}

// calculateEndPosition 计算分割结束位置
func calculateEndPosition(bytes []byte, start, maxLength int) int {
	end := start + maxLength
	if end >= len(bytes) {
		return len(bytes)
	}

	end = findValidUTF8Boundary(bytes, start, end)
	return end
}

// findValidUTF8Boundary 查找有效的 UTF-8 字符边界
func findValidUTF8Boundary(bytes []byte, start, end int) int {
	for end > start && (bytes[end]&0xC0) == 0x80 {
		end--
	}

	if end == start {
		end = skipUTF8ContinuationBytes(bytes, start)
	}

	return end
}

// skipUTF8ContinuationBytes 跳过 UTF-8 续字节
func skipUTF8ContinuationBytes(bytes []byte, start int) int {
	end := start
	for end < len(bytes) && (bytes[end]&0xC0) == 0x80 {
		end++
	}
	if end < len(bytes) {
		end++
	}
	return end
}

// GetPathSuffix 获取路径的文件后缀（不含点号）
func GetPathSuffix(path string) string {
	// 解析URL获取路径部分
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err == nil {
			path = u.Path
		}
	}
	ext := filepath.Ext(path)
	return strings.TrimPrefix(ext, ".")
}

// RemoveMarkdownSymbol 移除Markdown格式符号
// 目前仅移除 ** 包裹的文本
func RemoveMarkdownSymbol(text string) string {
	if text == "" {
		return text
	}
	// 移除 **text** 格式
	re := regexp.MustCompile(`\*\*(.*?)\*\*`)
	return re.ReplaceAllString(text, "$1")
}

// ExpandPath 展开路径中的 ~ 为用户主目录
// 支持跨平台（Windows/Unix）
func ExpandPath(path string) string {
	if path == "" {
		return path
	}

	// 先尝试标准展开
	expanded := os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				expanded = home
			} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
				expanded = filepath.Join(home, path[2:])
			}
		}
	}

	return expanded
}

// ContainsString 检查字符串切片是否包含指定字符串
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ContainsInt 检查整数切片是否包含指定整数
func ContainsInt(slice []int, n int) bool {
	for _, item := range slice {
		if item == n {
			return true
		}
	}
	return false
}

// RemoveString 从字符串切片中移除指定字符串
func RemoveString(slice []string, s string) []string {
	result := make([]string, 0)
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// UniqueString 去重字符串切片
func UniqueString(slice []string) []string {
	keys := make(map[string]bool)
	result := make([]string, 0)
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}

// MinInt 返回两个整数中的较小值
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxInt 返回两个整数中的较大值
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MinInt64 返回两个int64中的较小值
func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// MaxInt64 返回两个int64中的较大值
func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Ternary 三元运算符
func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

// DefaultIfEmpty 如果字符串为空则返回默认值
func DefaultIfEmpty(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

// Ptr 返回值的指针
func Ptr[T any](v T) *T {
	return &v
}

// ValueOrDefault 返回指针的值，如果指针为nil则返回零值
func ValueOrDefault[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// ValueOr 返回指针的值，如果指针为nil则返回指定默认值
func ValueOr[T any](p *T, defaultVal T) T {
	if p == nil {
		return defaultVal
	}
	return *p
}

// BoolToInt 布尔值转整数，true返回1，false返回0
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
