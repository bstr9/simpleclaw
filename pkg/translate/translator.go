// Package translate 提供翻译服务的抽象接口和实现。
// translator.go 定义了翻译器的核心接口。
package translate

// Translator 翻译器接口
// 定义了翻译服务的基本操作，支持多种翻译服务提供商
//
// 语言代码请参考 ISO 639-1 标准：https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes
// 不同翻译服务商可能有自己的语言代码映射规则
type Translator interface {
	// Translate 将文本从源语言翻译到目标语言
	//
	// 参数：
	//   - text: 需要翻译的文本内容
	//   - from: 源语言代码（ISO 639-1），空字符串表示自动检测
	//   - to: 目标语言代码（ISO 639-1）
	//
	// 返回：
	//   - string: 翻译后的文本
	//   - error: 翻译过程中的错误
	//
	// 示例：
	//   result, err := translator.Translate("你好世界", "zh", "en")
	//   result, err := translator.Translate("Hello", "", "zh") // 自动检测源语言
	Translate(text string, from string, to string) (string, error)
}
