// Package agent 提供 AI Agent 核心实现
// executor.go 实现执行循环和 Tool calling 处理
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// executor Agent 执行器
type executor struct {
	agent   *Agent
	onEvent func(event map[string]any)
}

// newExecutor 创建执行器
func newExecutor(a *Agent, onEvent func(event map[string]any)) *executor {
	return &executor{
		agent:   a,
		onEvent: onEvent,
	}
}

// run 执行 Agent 主循环
func (e *executor) run(ctx context.Context, userMessage string) (string, error) {
	logger.Info("Starting agent execution",
		zap.Int("max_steps", e.agent.maxSteps),
		zap.Int("tools_count", len(e.agent.tools)),
		zap.Bool("stream", e.agent.stream),
	)

	currentMessage := userMessage

	for step := 0; step < e.agent.maxSteps; step++ {
		emitEvent(e.onEvent, EventTypeStepStart, map[string]any{
			"step": step + 1,
		})

		logger.Info("Calling LLM", zap.Int("step", step+1))

		response, err := e.callLLMWithMode(ctx, currentMessage)
		if err != nil {
			return e.handleLLMError(err, step)
		}

		e.logResponse(response, step)

		if e.hasToolCalls(response) {
			return e.handleNoToolCalls(response, step)
		}

		e.agent.AddToolCallMessage(response.ToolCalls)

		if err := e.processToolCalls(ctx, response.ToolCalls); err != nil {
			return "", fmt.Errorf("tool call processing failed at step %d: %w", step+1, err)
		}

		currentMessage = ""

		emitEvent(e.onEvent, EventTypeStepEnd, map[string]any{
			"step":   step + 1,
			"status": "tool_calls_processed",
		})
	}

	return "", fmt.Errorf("max steps (%d) reached without completion", e.agent.maxSteps)
}

// callLLMWithMode 根据模式调用 LLM
func (e *executor) callLLMWithMode(ctx context.Context, message string) (*llm.Response, error) {
	if e.agent.stream && len(e.agent.tools) == 0 {
		return e.callLLMStreamMode(ctx, message)
	}
	return e.callLLM(ctx, message)
}

// handleLLMError 处理 LLM 调用错误
func (e *executor) handleLLMError(err error, step int) (string, error) {
	emitEvent(e.onEvent, EventTypeError, map[string]any{
		"error": err.Error(),
	})
	logger.Error("LLM call failed", zap.Error(err), zap.Int("step", step+1))
	return "", fmt.Errorf("LLM call failed at step %d: %w", step+1, err)
}

// logResponse 记录 LLM 响应日志
func (e *executor) logResponse(response *llm.Response, step int) {
	logger.Info("LLM response received",
		zap.Int("content_length", len(response.Content)),
		zap.Int("tool_calls_count", len(response.ToolCalls)),
		zap.String("finish_reason", response.FinishReason))

	if len(response.ToolCalls) == 0 && len(response.Content) > 0 {
		preview := response.Content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		logger.Warn("LLM returned content without tool calls",
			zap.String("content_preview", preview))
	}
}

// hasToolCalls 检查响应是否没有工具调用
func (e *executor) hasToolCalls(response *llm.Response) bool {
	return len(response.ToolCalls) == 0
}

// handleNoToolCalls 处理无工具调用的情况
func (e *executor) handleNoToolCalls(response *llm.Response, step int) (string, error) {
	stepNum := step + 1
	emitEvent(e.onEvent, EventTypeStepEnd, map[string]any{
		"step":   stepNum,
		"status": "completed",
	})

	emitEvent(e.onEvent, EventTypeComplete, map[string]any{
		"response": response.Content,
	})

	e.agent.AddAssistantMessage(response.Content)

	return response.Content, nil
}

// callLLM 调用 LLM
func (e *executor) callLLM(ctx context.Context, userMessage string) (*llm.Response, error) {
	logger.Debug("Calling LLM")

	opts := []llm.Option{
		llm.WithSystemPrompt(e.agent.systemPrompt),
	}

	if e.agent.maxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(e.agent.maxTokens))
	}
	if e.agent.temperature > 0 {
		temp := float64(e.agent.temperature)
		opts = append(opts, llm.WithTemperature(temp))
	}

	if len(e.agent.tools) > 0 {
		toolDefs := make([]llm.ToolDefinition, len(e.agent.tools))
		for i, tool := range e.agent.tools {
			toolDefs[i] = llm.ToolDefinition{
				Type: "function",
				Function: llm.FunctionDefinition{
					Name:        tool.Name(),
					Description: tool.Description(),
					Parameters:  tool.Parameters(),
				},
			}
		}
		opts = append(opts, llm.WithTools(toolDefs))
		logger.Info("Sending tools to LLM",
			zap.Int("count", len(toolDefs)),
			zap.Strings("names", func() []string {
				names := make([]string, len(toolDefs))
				for i, t := range toolDefs {
					names[i] = t.Function.Name
				}
				return names
			}()))
	}

	messages := e.agent.GetMessagesWithSystem()
	if userMessage != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: userMessage,
		})
	}

	response, err := e.agent.model.Call(ctx, messages, opts...)
	if err != nil {
		logger.Error("LLM call failed", zap.Error(err))
		return nil, err
	}

	logger.Debug("LLM response received",
		zap.Int("content_length", len(response.Content)),
		zap.Int("tool_calls_count", len(response.ToolCalls)),
	)

	return response, nil
}

// callLLMStreamMode 调用 LLM (流式模式)
func (e *executor) callLLMStreamMode(ctx context.Context, userMessage string) (*llm.Response, error) {
	logger.Debug("Calling LLM (stream mode)")

	opts := []llm.Option{
		llm.WithSystemPrompt(e.agent.systemPrompt),
	}
	if e.agent.maxTokens > 0 {
		opts = append(opts, llm.WithMaxTokens(e.agent.maxTokens))
	}
	if e.agent.temperature > 0 {
		opts = append(opts, llm.WithTemperature(float64(e.agent.temperature)))
	}

	messages := e.agent.GetMessagesWithSystem()
	if userMessage != "" {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: userMessage,
		})
	}

	stream, err := e.agent.model.CallStream(ctx, messages, opts...)
	if err != nil {
		logger.Error("LLM stream call failed", zap.Error(err))
		return nil, err
	}

	var contentBuilder strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			logger.Error("Stream error", zap.Error(chunk.Error))
			return &llm.Response{Content: contentBuilder.String()}, chunk.Error
		}
		if chunk.Delta != "" {
			contentBuilder.WriteString(chunk.Delta)
			emitEvent(e.onEvent, EventTypeText, map[string]any{
				"delta": chunk.Delta,
			})
		}
		if chunk.Done {
			break
		}
	}

	return &llm.Response{Content: contentBuilder.String()}, nil
}

type toolResult struct {
	id     string
	result string
	status string
	err    error
}

// processToolCalls 处理工具调用（并行执行）
func (e *executor) processToolCalls(ctx context.Context, toolCalls []llm.ToolCall) error {
	e.agent.AddToolCallMessage(toolCalls)

	results := make([]toolResult, len(toolCalls))
	var wg sync.WaitGroup

	for i, toolCall := range toolCalls {
		wg.Add(1)
		go func(idx int, tc llm.ToolCall) {
			defer wg.Done()

			logger.Info("Processing tool call",
				zap.String("tool_name", tc.Function.Name),
				zap.String("tool_id", tc.ID))

			emitEvent(e.onEvent, EventTypeToolCall, map[string]any{
				"tool_name": tc.Function.Name,
				"tool_id":   tc.ID,
				"arguments": tc.Function.Arguments,
			})

			result, err := e.executeTool(ctx, tc)
			if err != nil {
				errorResult := NewErrorToolResult(err)
				results[idx] = toolResult{
					id:     tc.ID,
					result: errorResult.Result.(string),
					status: "error",
					err:    err,
				}

				emitEvent(e.onEvent, EventTypeToolResult, map[string]any{
					"tool_id": tc.ID,
					"status":  "error",
					"error":   err.Error(),
				})

				logger.Error("Tool execution failed",
					zap.String("tool_name", tc.Function.Name),
					zap.Error(err))
				return
			}

			resultJSON, _ := json.Marshal(result.Result)
			results[idx] = toolResult{
				id:     tc.ID,
				result: string(resultJSON),
				status: result.Status,
			}

			emitEvent(e.onEvent, EventTypeToolResult, map[string]any{
				"tool_id": tc.ID,
				"status":  result.Status,
				"result":  result.Result,
			})

			logger.Info("Tool execution completed",
				zap.String("tool_name", tc.Function.Name),
				zap.String("status", result.Status))
		}(i, toolCall)
	}

	wg.Wait()

	// 按原始顺序添加消息结果
	errorCount := 0
	for _, r := range results {
		e.agent.AddToolResultMessage(r.id, r.result)
		if r.err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("%d tool calls failed", errorCount)
	}
	return nil
}

// executeTool 执行单个工具
func (e *executor) executeTool(ctx context.Context, toolCall llm.ToolCall) (*ToolResult, error) {
	tool, ok := e.agent.toolRegistry.Get(toolCall.Function.Name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", toolCall.Function.Name)
	}

	params, err := parseToolCallArgs(toolCall)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	if tool.Stage() == ToolStagePreProcess {
		logger.Debug("Tool is pre-process stage, skipping in post-process context",
			zap.String("tool_name", tool.Name()),
		)
	}

	if toolCtx, ok := tool.(ToolWithContext); ok && e.agent.toolCtx != nil {
		return toolCtx.ExecuteWithContext(e.agent.toolCtx, params)
	}

	return tool.Execute(params)
}
