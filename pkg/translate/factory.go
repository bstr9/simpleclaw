// Package translate 提供翻译服务的抽象接口和实现。
// factory.go 定义了翻译器工厂和注册机制。
package translate

import (
	"fmt"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// TranslatorCreator 翻译器创建函数类型
type TranslatorCreator func() (Translator, error)

// translatorRegistry 翻译器注册表
var translatorRegistry = struct {
	mu       sync.RWMutex
	creators map[string]TranslatorCreator
}{
	creators: make(map[string]TranslatorCreator),
}

// 预定义的翻译器类型常量
const (
	// TranslatorBaidu 百度翻译
	TranslatorBaidu = "baidu"
	// TranslatorGoogle Google翻译（预留）
	TranslatorGoogle = "google"
	// TranslatorDeepL DeepL翻译（预留）
	TranslatorDeepL = "deepl"
	// TranslatorYoudao 有道翻译（预留）
	TranslatorYoudao = "youdao"
)

// RegisterTranslator 注册翻译器创建函数
//
// 参数：
//   - name: 翻译器类型名称（如 "baidu", "google" 等）
//   - creator: 翻译器创建函数
//
// 示例：
//
//	RegisterTranslator("baidu", func() (Translator, error) {
//	    return NewBaiduTranslator()
//	})
func RegisterTranslator(name string, creator TranslatorCreator) {
	translatorRegistry.mu.Lock()
	defer translatorRegistry.mu.Unlock()

	if _, exists := translatorRegistry.creators[name]; exists {
		logger.Warn("[TranslatorFactory] 覆盖已存在的翻译器注册",
			zap.String("translator", name))
	}

	translatorRegistry.creators[name] = creator
	logger.Debug("[TranslatorFactory] 翻译器已注册",
		zap.String("translator", name))
}

// CreateTranslator 创建翻译器实例
//
// 参数：
//   - translatorType: 翻译器类型名称
//
// 返回：
//   - Translator: 翻译器实例
//   - error: 创建失败时的错误信息
func CreateTranslator(translatorType string) (Translator, error) {
	translatorRegistry.mu.RLock()
	creator, exists := translatorRegistry.creators[translatorType]
	translatorRegistry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("未知的翻译器类型: %s", translatorType)
	}

	translator, err := creator()
	if err != nil {
		return nil, fmt.Errorf("创建翻译器 '%s' 失败: %w", translatorType, err)
	}

	return translator, nil
}

// GetRegisteredTranslators 获取已注册的翻译器类型列表
func GetRegisteredTranslators() []string {
	translatorRegistry.mu.RLock()
	defer translatorRegistry.mu.RUnlock()

	types := make([]string, 0, len(translatorRegistry.creators))
	for name := range translatorRegistry.creators {
		types = append(types, name)
	}

	return types
}

// IsTranslatorRegistered 检查翻译器是否已注册
func IsTranslatorRegistered(name string) bool {
	translatorRegistry.mu.RLock()
	defer translatorRegistry.mu.RUnlock()
	_, exists := translatorRegistry.creators[name]
	return exists
}

// translatorCache 翻译器实例缓存
var translatorCache = struct {
	mu        sync.RWMutex
	instances map[string]Translator
}{
	instances: make(map[string]Translator),
}

// GetCachedTranslator 获取缓存的翻译器实例
func GetCachedTranslator(name string) Translator {
	translatorCache.mu.RLock()
	defer translatorCache.mu.RUnlock()
	return translatorCache.instances[name]
}

// SetCachedTranslator 缓存翻译器实例
func SetCachedTranslator(name string, translator Translator) {
	translatorCache.mu.Lock()
	defer translatorCache.mu.Unlock()
	translatorCache.instances[name] = translator
}

// ClearTranslatorCache 清除翻译器缓存
func ClearTranslatorCache(name string) {
	translatorCache.mu.Lock()
	defer translatorCache.mu.Unlock()

	if _, exists := translatorCache.instances[name]; exists {
		delete(translatorCache.instances, name)
		logger.Debug("[TranslatorFactory] 翻译器缓存已清除",
			zap.String("translator", name))
	}
}

// ClearAllTranslatorCache 清除所有翻译器缓存
func ClearAllTranslatorCache() {
	translatorCache.mu.Lock()
	defer translatorCache.mu.Unlock()
	translatorCache.instances = make(map[string]Translator)
	logger.Debug("[TranslatorFactory] 所有翻译器缓存已清除")
}

// TranslatorBuilder 翻译器构建器
// 提供链式 API 创建翻译器
type TranslatorBuilder struct {
	translatorType string
	aliases        []string
	creator        TranslatorCreator
}

// NewTranslatorBuilder 创建翻译器构建器
func NewTranslatorBuilder(translatorType string) *TranslatorBuilder {
	return &TranslatorBuilder{
		translatorType: translatorType,
	}
}

// WithAliases 设置别名
func (b *TranslatorBuilder) WithAliases(aliases ...string) *TranslatorBuilder {
	b.aliases = append(b.aliases, aliases...)
	return b
}

// WithCreator 设置创建函数
func (b *TranslatorBuilder) WithCreator(creator TranslatorCreator) *TranslatorBuilder {
	b.creator = creator
	return b
}

// Register 注册翻译器
func (b *TranslatorBuilder) Register() error {
	if b.creator == nil {
		return fmt.Errorf("翻译器 '%s' 需要创建函数", b.translatorType)
	}

	// 注册主翻译器
	RegisterTranslator(b.translatorType, b.creator)

	// 注册别名
	for _, alias := range b.aliases {
		RegisterTranslator(alias, b.creator)
		logger.Debug("[TranslatorFactory] 翻译器别名已注册",
			zap.String("alias", alias),
			zap.String("target", b.translatorType))
	}

	return nil
}

// init 初始化默认翻译器注册
func init() {
	// 注册占位符创建函数，具体实现由各翻译器包在导入时注册
	placeholders := []string{
		TranslatorBaidu,
		TranslatorGoogle,
		TranslatorDeepL,
		TranslatorYoudao,
	}

	for _, name := range placeholders {
		translatorName := name // 捕获循环变量
		RegisterTranslator(name, func() (Translator, error) {
			return nil, fmt.Errorf("翻译器 '%s' 尚未实现", translatorName)
		})
	}
}
