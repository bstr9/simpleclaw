// Package streaming 提供流式输出到渠道的支持
package streaming

// StreamWriter 流式写入器接口
type StreamWriter interface {
	Write(chunk string) error
	Flush() error
	Close() error
}
