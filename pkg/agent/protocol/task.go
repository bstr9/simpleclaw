// Package protocol 提供 Agent 交互协议的核心定义。
// task.go 定义了任务相关的类型和接口。
package protocol

import (
	"maps"
	"time"

	"github.com/google/uuid"
)

// TaskType 定义任务类型枚举
type TaskType string

const (
	// TaskTypeText 文本任务
	TaskTypeText TaskType = "text"
	// TaskTypeImage 图片任务
	TaskTypeImage TaskType = "image"
	// TaskTypeVideo 视频任务
	TaskTypeVideo TaskType = "video"
	// TaskTypeAudio 音频任务
	TaskTypeAudio TaskType = "audio"
	// TaskTypeFile 文件任务
	TaskTypeFile TaskType = "file"
	// TaskTypeMixed 混合类型任务
	TaskTypeMixed TaskType = "mixed"
)

// String 返回 TaskType 的字符串表示
func (t TaskType) String() string {
	return string(t)
}

// TaskStatus 定义任务状态枚举
type TaskStatus string

const (
	// TaskStatusInit 初始状态
	TaskStatusInit TaskStatus = "init"
	// TaskStatusProcessing 处理中
	TaskStatusProcessing TaskStatus = "processing"
	// TaskStatusCompleted 已完成
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 已失败
	TaskStatusFailed TaskStatus = "failed"
)

// String 返回 TaskStatus 的字符串表示
func (s TaskStatus) String() string {
	return string(s)
}

// IsTerminal 检查任务是否处于终态
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed
}

// Task 表示 Agent 需要处理的任务
type Task struct {
	// ID 任务的唯一标识符
	ID string `json:"id"`
	// Content 任务的主要文本内容
	Content string `json:"content"`
	// Type 任务类型
	Type TaskType `json:"type"`
	// Status 当前任务状态
	Status TaskStatus `json:"status"`
	// CreatedAt 任务创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 任务最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// Metadata 任务的额外元数据
	Metadata map[string]any `json:"metadata,omitempty"`

	// Media 媒体内容
	// Images 图片 URL 或 base64 编码列表
	Images []string `json:"images,omitempty"`
	// Videos 视频 URL 列表
	Videos []string `json:"videos,omitempty"`
	// Audios 音频 URL 或 base64 编码列表
	Audios []string `json:"audios,omitempty"`
	// Files 文件 URL 或路径列表
	Files []string `json:"files,omitempty"`
}

// NewTask 创建新的任务实例
func NewTask(content string, opts ...TaskOption) *Task {
	now := time.Now()
	task := &Task{
		ID:        uuid.New().String(),
		Content:   content,
		Type:      TaskTypeText,
		Status:    TaskStatusInit,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]any),
		Images:    make([]string, 0),
		Videos:    make([]string, 0),
		Audios:    make([]string, 0),
		Files:     make([]string, 0),
	}

	for _, opt := range opts {
		opt(task)
	}

	return task
}

// TaskOption 任务配置选项函数
type TaskOption func(*Task)

// WithTaskType 设置任务类型
func WithTaskType(t TaskType) TaskOption {
	return func(task *Task) {
		task.Type = t
	}
}

// WithTaskStatus 设置任务状态
func WithTaskStatus(s TaskStatus) TaskOption {
	return func(task *Task) {
		task.Status = s
	}
}

// WithTaskMetadata 设置任务元数据
func WithTaskMetadata(metadata map[string]any) TaskOption {
	return func(task *Task) {
		if metadata != nil {
			task.Metadata = metadata
		}
	}
}

// WithTaskImages 设置任务图片
func WithTaskImages(images []string) TaskOption {
	return func(task *Task) {
		task.Images = images
	}
}

// WithTaskVideos 设置任务视频
func WithTaskVideos(videos []string) TaskOption {
	return func(task *Task) {
		task.Videos = videos
	}
}

// WithTaskAudios 设置任务音频
func WithTaskAudios(audios []string) TaskOption {
	return func(task *Task) {
		task.Audios = audios
	}
}

// WithTaskFiles 设置任务文件
func WithTaskFiles(files []string) TaskOption {
	return func(task *Task) {
		task.Files = files
	}
}

// WithTaskID 设置任务 ID
func WithTaskID(id string) TaskOption {
	return func(task *Task) {
		task.ID = id
	}
}

// GetText 获取任务的文本内容
func (t *Task) GetText() string {
	return t.Content
}

// UpdateStatus 更新任务状态
func (t *Task) UpdateStatus(status TaskStatus) {
	t.Status = status
	t.UpdatedAt = time.Now()
}

// SetMetadata 设置元数据项
func (t *Task) SetMetadata(key string, value any) {
	if t.Metadata == nil {
		t.Metadata = make(map[string]any)
	}
	t.Metadata[key] = value
	t.UpdatedAt = time.Now()
}

// GetMetadata 获取元数据项
func (t *Task) GetMetadata(key string) (any, bool) {
	if t.Metadata == nil {
		return nil, false
	}
	val, ok := t.Metadata[key]
	return val, ok
}

// AddImage 添加图片
func (t *Task) AddImage(image string) {
	t.Images = append(t.Images, image)
	t.UpdatedAt = time.Now()
}

// AddVideo 添加视频
func (t *Task) AddVideo(video string) {
	t.Videos = append(t.Videos, video)
	t.UpdatedAt = time.Now()
}

// AddAudio 添加音频
func (t *Task) AddAudio(audio string) {
	t.Audios = append(t.Audios, audio)
	t.UpdatedAt = time.Now()
}

// AddFile 添加文件
func (t *Task) AddFile(file string) {
	t.Files = append(t.Files, file)
	t.UpdatedAt = time.Now()
}

// HasMedia 检查任务是否包含媒体内容
func (t *Task) HasMedia() bool {
	return len(t.Images) > 0 || len(t.Videos) > 0 || len(t.Audios) > 0 || len(t.Files) > 0
}

// Clone 创建任务的深拷贝
func (t *Task) Clone() *Task {
	metadata := make(map[string]any)
	maps.Copy(metadata, t.Metadata)

	return &Task{
		ID:        t.ID,
		Content:   t.Content,
		Type:      t.Type,
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
		Metadata:  metadata,
		Images:    append([]string{}, t.Images...),
		Videos:    append([]string{}, t.Videos...),
		Audios:    append([]string{}, t.Audios...),
		Files:     append([]string{}, t.Files...),
	}
}
