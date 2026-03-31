// Package common 提供时间检查器
package common

import (
	"fmt"
	"regexp"
	"time"
)

// TimeChecker 时间检查器
// 用于检查当前时间是否在服务时间范围内
type TimeChecker struct {
	enabled   bool           // 是否启用时间检查
	startTime string         // 服务开始时间 (HH:MM)
	stopTime  string         // 服务结束时间 (HH:MM)
	timeRegex *regexp.Regexp // 时间格式正则
	debugMode bool           // 调试模式
}

// TimeCheckerOption 时间检查器配置选项
type TimeCheckerOption func(*TimeChecker)

// WithTimeRange 设置服务时间范围
func WithTimeRange(startTime, stopTime string) TimeCheckerOption {
	return func(tc *TimeChecker) {
		tc.startTime = startTime
		tc.stopTime = stopTime
	}
}

// WithEnabled 设置是否启用
func WithEnabled(enabled bool) TimeCheckerOption {
	return func(tc *TimeChecker) {
		tc.enabled = enabled
	}
}

// WithDebugMode 设置调试模式
func WithDebugMode(debug bool) TimeCheckerOption {
	return func(tc *TimeChecker) {
		tc.debugMode = debug
	}
}

// NewTimeChecker 创建新的时间检查器
func NewTimeChecker(opts ...TimeCheckerOption) *TimeChecker {
	tc := &TimeChecker{
		enabled:   false,
		startTime: "00:00",
		stopTime:  "24:00",
		timeRegex: regexp.MustCompile(`^([01]?[0-9]|2[0-4])(:)([0-5][0-9])$`),
		debugMode: false,
	}

	for _, opt := range opts {
		opt(tc)
	}

	return tc
}

// IsEnabled 检查是否启用时间检查
func (tc *TimeChecker) IsEnabled() bool {
	return tc.enabled
}

// SetEnabled 设置是否启用时间检查
func (tc *TimeChecker) SetEnabled(enabled bool) {
	tc.enabled = enabled
}

// SetTimeRange 设置服务时间范围
func (tc *TimeChecker) SetTimeRange(startTime, stopTime string) error {
	if !tc.timeRegex.MatchString(startTime) || !tc.timeRegex.MatchString(stopTime) {
		return fmt.Errorf("时间格式不正确，请使用 HH:MM 格式")
	}
	tc.startTime = startTime
	tc.stopTime = stopTime
	return nil
}

// GetTimeRange 获取服务时间范围
func (tc *TimeChecker) GetTimeRange() (string, string) {
	return tc.startTime, tc.stopTime
}

// IsInServiceTime 检查当前时间是否在服务时间内
func (tc *TimeChecker) IsInServiceTime() bool {
	if !tc.enabled {
		return true
	}

	return tc.IsTimeInServiceRange(time.Now())
}

// IsTimeInServiceRange 检查指定时间是否在服务时间范围内
func (tc *TimeChecker) IsTimeInServiceRange(t time.Time) bool {
	if !tc.timeRegex.MatchString(tc.startTime) || !tc.timeRegex.MatchString(tc.stopTime) {
		if tc.debugMode {
			fmt.Printf("[TimeChecker] 时间格式不正确: start=%s, stop=%s\n", tc.startTime, tc.stopTime)
		}
		return true
	}

	nowTime, _ := time.Parse("15:04", t.Format("15:04"))
	startTime, _ := time.Parse("15:04", tc.startTime)
	stopTime, _ := time.Parse("15:04", tc.stopTime)

	if stopTime.Before(startTime) {
		return nowTime.After(startTime) || nowTime.Before(stopTime) || nowTime.Equal(startTime) || nowTime.Equal(stopTime)
	}

	return (nowTime.After(startTime) || nowTime.Equal(startTime)) &&
		(nowTime.Before(stopTime) || nowTime.Equal(stopTime))
}

// CheckAndExecute 检查服务时间并执行函数
func (tc *TimeChecker) CheckAndExecute(fn func() error) error {
	if tc.IsInServiceTime() {
		return fn()
	}
	return ErrNotInServiceTime
}

// ErrNotInServiceTime 非服务时间错误
var ErrNotInServiceTime = NewError("非服务时间内，不接受访问")

// TimeCheckFunc 时间检查函数类型
type TimeCheckFunc func() bool

// WrapWithTimeCheck 包装函数，在服务时间内才执行
func (tc *TimeChecker) WrapWithTimeCheck(fn func() error) func() error {
	return func() error {
		if !tc.IsInServiceTime() {
			return ErrNotInServiceTime
		}
		return fn()
	}
}

// ParseTimeRange 解析时间范围字符串 (格式: HH:MM-HH:MM)
func ParseTimeRange(rangeStr string) (start, stop string, err error) {
	re := regexp.MustCompile(`^(\d{1,2}:\d{2})-(\d{1,2}:\d{2})$`)
	matches := re.FindStringSubmatch(rangeStr)
	if matches == nil {
		return "", "", fmt.Errorf("无效的时间范围格式，请使用 HH:MM-HH:MM")
	}
	return matches[1], matches[2], nil
}

// GetRemainingTime 获取距离下次服务时间的剩余时长
func (tc *TimeChecker) GetRemainingTime() time.Duration {
	if tc.IsInServiceTime() {
		return 0
	}

	now := time.Now()
	nowTime, _ := time.Parse("15:04", now.Format("15:04"))
	startTime, _ := time.Parse("15:04", tc.startTime)

	todayStart := time.Date(now.Year(), now.Month(), now.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, now.Location())

	var nextStart time.Time
	if nowTime.Before(startTime) {
		nextStart = todayStart
	} else {
		nextStart = todayStart.Add(24 * time.Hour)
	}

	return nextStart.Sub(now)
}

// GetServiceDuration 获取服务时长
func (tc *TimeChecker) GetServiceDuration() time.Duration {
	startTime, _ := time.Parse("15:04", tc.startTime)
	stopTime, _ := time.Parse("15:04", tc.stopTime)

	if stopTime.Before(startTime) {
		midnight, _ := time.Parse("15:04", "24:00")
		return (midnight.Sub(startTime) + stopTime.Sub(time.Time{}))
	}

	return stopTime.Sub(startTime)
}

// FormatTimeFormat 格式化时间为 HH:MM 格式
func FormatTimeFormat(t time.Time) string {
	return t.Format("15:04")
}

// ParseTimeString 解析时间字符串 (格式: HH:MM)
func ParseTimeString(timeStr string) (time.Time, error) {
	return time.Parse("15:04", timeStr)
}
