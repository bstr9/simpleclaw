// Package streaming 提供流式输出到渠道的支持
//
// Deprecated: 此包当前未被使用。流式输出通过 AgentBridge 的 onEvent 回调
// 和 pkg/api/server.go 的 SSE 实现处理。保留此包作为未来统一流式接口的参考。
package streaming

// StreamWriter 流式写入器接口
type StreamWriter interface {
	Write(chunk string) error
	Flush() error
	Close() error
}
