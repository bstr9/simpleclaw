// Package tool 提供工具调用插件，支持多种工具来增强 AI 机器人的能力。
// 该文件包含内置工具的实现，包括 URL 获取、天气查询、计算器和搜索。
package tool

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ===== URL 获取工具 =====

// URLGetTool URL 获取工具。
type URLGetTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *URLGetTool) Name() string {
	return ToolURLGet
}

// Description 返回工具描述。
func (t *URLGetTool) Description() string {
	return "获取 URL 内容，用于访问网页并提取文本内容"
}

// Run 执行工具。
func (t *URLGetTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 60
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	// 提取 URL
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要获取的 URL")
	}

	// 验证 URL
	parsedURL, err := url.Parse(query)
	if err != nil {
		return "", fmt.Errorf("无效的 URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("仅支持 HTTP 和 HTTPS 协议")
	}

	// 创建请求
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 提取文本内容（简单实现，移除 HTML 标签）
	content := string(body)
	content = t.extractText(content)

	// 限制返回内容长度
	maxLength := 4000
	if len(content) > maxLength {
		content = content[:maxLength] + "\n... (内容已截断)"
	}

	return fmt.Sprintf("URL: %s\n\n%s", query, content), nil
}

// extractText 从 HTML 中提取文本内容。
func (t *URLGetTool) extractText(html string) string {
	// 移除 script 和 style 标签及其内容
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = scriptRegex.ReplaceAllString(html, "")
	html = styleRegex.ReplaceAllString(html, "")

	// 移除所有 HTML 标签
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, "")

	// 解码 HTML 实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// 清理多余空白
	whitespaceRegex := regexp.MustCompile(`\s+`)
	text = whitespaceRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// ===== 天气工具 =====

// MeteoTool 天气工具，使用 Open-Meteo API。
type MeteoTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *MeteoTool) Name() string {
	return "meteo"
}

// Description 返回工具描述。
func (t *MeteoTool) Description() string {
	return "查询天气信息，支持查询任意城市的当前天气和天气预报"
}

// Run 执行工具。
func (t *MeteoTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 30
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要查询的城市名称")
	}

	// 解析城市名称（支持中文格式）
	city := t.extractCity(query)

	// 步骤1：获取城市的经纬度
	lat, lon, locationName, err := t.getGeocoding(city)
	if err != nil {
		return "", fmt.Errorf("获取城市坐标失败: %w", err)
	}

	// 步骤2：获取天气数据
	weatherData, err := t.getWeather(lat, lon)
	if err != nil {
		return "", fmt.Errorf("获取天气数据失败: %w", err)
	}

	// 格式化输出
	return t.formatWeatherResponse(locationName, weatherData), nil
}

// extractCity 从查询中提取城市名称。
func (t *MeteoTool) extractCity(query string) string {
	// 移除常见的查询词
	replacements := []string{
		"今天", "明天", "后天", "大后天",
		"的天气", "天气", "天气预报",
		"怎么样", "如何", "怎样",
		"查询", "查一下", "看看",
		"？", "?",
	}

	city := query
	for _, r := range replacements {
		city = strings.ReplaceAll(city, r, "")
	}

	return strings.TrimSpace(city)
}

// getGeocoding 获取城市的经纬度。
func (t *MeteoTool) getGeocoding(city string) (lat, lon float64, name string, err error) {
	// 使用 Open-Meteo Geocoding API
	apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json",
		url.QueryEscape(city))

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", err
	}

	// 解析响应
	var geocodingResp struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &geocodingResp); err != nil {
		return 0, 0, "", err
	}

	if len(geocodingResp.Results) == 0 {
		return 0, 0, "", fmt.Errorf("未找到城市: %s", city)
	}

	result := geocodingResp.Results[0]
	locationName := result.Name
	if result.Admin1 != "" {
		locationName = result.Admin1 + ", " + result.Name
	}
	if result.Country != "" {
		locationName = result.Name + ", " + result.Country
	}

	return result.Latitude, result.Longitude, locationName, nil
}

// getWeather 获取天气数据。
func (t *MeteoTool) getWeather(lat, lon float64) (map[string]any, error) {
	// 使用 Open-Meteo Weather API
	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m,wind_direction_10m&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=3",
		lat, lon)

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var weatherData map[string]any
	if err := json.Unmarshal(body, &weatherData); err != nil {
		return nil, err
	}

	return weatherData, nil
}

// formatWeatherResponse 格式化天气响应。
func (t *MeteoTool) formatWeatherResponse(locationName string, data map[string]any) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📍 地点: %s\n\n", locationName))

	t.formatCurrentWeather(data, &sb)
	t.formatDailyForecast(data, &sb)

	return sb.String()
}

// formatCurrentWeather 格式化当前天气
func (t *MeteoTool) formatCurrentWeather(data map[string]any, sb *strings.Builder) {
	current, ok := data["current"].(map[string]any)
	if !ok {
		return
	}

	sb.WriteString("🌡️ 当前天气:\n")
	if temp, ok := current["temperature_2m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  温度: %.1f°C\n", temp))
	}
	if apparent, ok := current["apparent_temperature"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  体感温度: %.1f°C\n", apparent))
	}
	if humidity, ok := current["relative_humidity_2m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  湿度: %.0f%%\n", humidity))
	}
	if windSpeed, ok := current["wind_speed_10m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  风速: %.1f km/h\n", windSpeed))
	}
	if weatherCode, ok := current["weather_code"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  天气: %s\n", t.getWeatherDescription(int(weatherCode))))
	}
}

// formatDailyForecast 格式化每日天气预报
func (t *MeteoTool) formatDailyForecast(data map[string]any, sb *strings.Builder) {
	daily, ok := data["daily"].(map[string]any)
	if !ok {
		return
	}

	times, ok := daily["time"].([]any)
	if !ok {
		return
	}

	sb.WriteString("\n📅 未来天气预报:\n")
	maxTemps, _ := daily["temperature_2m_max"].([]any)
	minTemps, _ := daily["temperature_2m_min"].([]any)
	weatherCodes, _ := daily["weather_code"].([]any)

	for i, time := range times {
		if i >= 3 {
			break
		}
		t.formatDailyEntry(i, time, maxTemps, minTemps, weatherCodes, sb)
	}
}

// formatDailyEntry 格式化单日天气预报
func (t *MeteoTool) formatDailyEntry(i int, time any, maxTemps, minTemps, weatherCodes []any, sb *strings.Builder) {
	date := time.(string)
	maxTemp := t.getArrayValue(maxTemps, i)
	minTemp := t.getArrayValue(minTemps, i)
	weather := t.getWeatherDescFromArray(weatherCodes, i)
	sb.WriteString(fmt.Sprintf("  %s: %s ~ %s°C, %s\n", date, minTemp, maxTemp, weather))
}

// getArrayValue 安全获取数组元素并格式化
func (t *MeteoTool) getArrayValue(arr []any, idx int) string {
	if idx >= len(arr) {
		return ""
	}
	if val, ok := arr[idx].(float64); ok {
		return fmt.Sprintf("%.1f", val)
	}
	return ""
}

// getWeatherDescFromArray 从数组获取天气描述
func (t *MeteoTool) getWeatherDescFromArray(arr []any, idx int) string {
	if idx >= len(arr) {
		return ""
	}
	if val, ok := arr[idx].(float64); ok {
		return t.getWeatherDescription(int(val))
	}
	return ""
}

// getWeatherDescription 根据天气代码返回描述。
func (t *MeteoTool) getWeatherDescription(code int) string {
	weatherCodes := map[int]string{
		0: "晴朗",
		1: "大部晴朗", 2: "多云", 3: "阴天",
		45: "雾", 48: "雾凇",
		51: "小毛毛雨", 53: "中毛毛雨", 55: "大毛毛雨",
		56: "冻毛毛雨", 57: "冻毛毛雨",
		61: "小雨", 63: "中雨", 65: "大雨",
		66: "冻雨", 67: "冻雨",
		71: "小雪", 73: "中雪", 75: "大雪",
		77: "雪粒",
		80: "小阵雨", 81: "中阵雨", 82: "大阵雨",
		85: "小阵雪", 86: "大阵雪",
		95: "雷暴",
		96: "雷暴伴小冰雹", 99: "雷暴伴大冰雹",
	}

	if desc, ok := weatherCodes[code]; ok {
		return desc
	}
	return fmt.Sprintf("天气代码 %d", code)
}

// ===== 计算器工具 =====

// CalculatorTool 计算器工具。
type CalculatorTool struct{}

// Name 返回工具名称。
func (t *CalculatorTool) Name() string {
	return "calculator"
}

// Description 返回工具描述。
func (t *CalculatorTool) Description() string {
	return "执行数学计算，支持加减乘除、幂运算、括号和数学函数"
}

// Run 执行工具。
func (t *CalculatorTool) Run(query string, config map[string]any) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要计算的表达式")
	}

	// 清理表达式
	expr := t.cleanExpression(query)

	// 解析并计算表达式
	result, err := t.evaluate(expr)
	if err != nil {
		return "", fmt.Errorf("计算错误: %w", err)
	}

	return fmt.Sprintf("计算结果: %s = %v", query, result), nil
}

// cleanExpression 清理表达式。
func (t *CalculatorTool) cleanExpression(expr string) string {
	// 移除中文运算符和常见描述
	replacements := map[string]string{
		"加": "+", "减": "-", "乘": "*", "除": "/",
		"等于": "=", "是多少": "", "计算": "",
		"请问": "", "帮我": "", "算一下": "",
		"多少": "", "？": "", "?": "",
		"×": "*", "÷": "/", "－": "-", "＋": "+",
		"（": "(", "）": ")",
	}

	result := expr
	for old, newStr := range replacements {
		result = strings.ReplaceAll(result, old, newStr)
	}

	// 移除所有空白字符
	result = strings.ReplaceAll(result, " ", "")
	result = strings.ReplaceAll(result, "\t", "")
	result = strings.ReplaceAll(result, "\n", "")

	return result
}

// evaluate 计算表达式。
func (t *CalculatorTool) evaluate(expr string) (float64, error) {
	// 使用简单的表达式解析器
	// 支持加减乘除、括号和幂运算

	// 预处理：将 ^ 替换为幂运算标记
	expr = strings.ReplaceAll(expr, "^", "**")

	// 解析并计算
	return t.parseExpression(expr)
}

// parseExpression 解析表达式（递归下降解析器）。
func (t *CalculatorTool) parseExpression(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("空表达式")
	}

	if t.hasMathFunction(expr) {
		return t.evaluateWithFunctions(expr)
	}

	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num, nil
	}

	expr, err := t.evaluateParentheses(expr)
	if err != nil {
		return 0, err
	}

	if result, ok, err := t.tryEvaluatePower(expr); ok {
		return result, err
	}

	if result, ok, err := t.tryEvaluateAddSub(expr); ok {
		return result, err
	}

	if result, ok, err := t.tryEvaluateMulDiv(expr); ok {
		return result, err
	}

	return strconv.ParseFloat(expr, 64)
}

// hasMathFunction 检查表达式是否包含数学函数。
func (t *CalculatorTool) hasMathFunction(expr string) bool {
	functions := []string{"sin(", "cos(", "tan(", "sqrt(", "log(", "ln(", "abs("}
	for _, f := range functions {
		if strings.Contains(expr, f) {
			return true
		}
	}
	return false
}

// evaluateParentheses 处理括号表达式，返回简化后的表达式。
func (t *CalculatorTool) evaluateParentheses(expr string) (string, error) {
	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		if start == -1 {
			break
		}
		end := strings.Index(expr[start:], ")")
		if end == -1 {
			return "", fmt.Errorf("括号不匹配")
		}
		end += start

		inner := expr[start+1 : end]
		result, err := t.parseExpression(inner)
		if err != nil {
			return "", err
		}

		expr = expr[:start] + fmt.Sprintf("%g", result) + expr[end+1:]
	}
	return expr, nil
}

// tryEvaluatePower 尝试处理幂运算。
func (t *CalculatorTool) tryEvaluatePower(expr string) (float64, bool, error) {
	if strings.Index(expr, "**") <= 0 {
		return 0, false, nil
	}

	parts := strings.Split(expr, "**")
	if len(parts) < 2 {
		return 0, false, nil
	}

	result, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return 0, true, fmt.Errorf("无效的操作数: %s", parts[len(parts)-1])
	}

	for i := len(parts) - 2; i >= 0; i-- {
		base, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			return 0, true, fmt.Errorf("无效的操作数: %s", parts[i])
		}
		result = t.pow(base, result)
	}
	return result, true, nil
}

// tryEvaluateAddSub 尝试处理加法和减法。
func (t *CalculatorTool) tryEvaluateAddSub(expr string) (float64, bool, error) {
	for i := len(expr) - 1; i >= 0; i-- {
		c := expr[i]
		if !t.isAddSubOperator(expr, i) {
			continue
		}

		if result, ok, err := t.evaluateBinaryOp(c, expr[:i], expr[i+1:]); ok {
			return result, true, err
		}
	}
	return 0, false, nil
}

// isAddSubOperator 检查位置 i 是否为有效的加减运算符。
func (t *CalculatorTool) isAddSubOperator(expr string, i int) bool {
	c := expr[i]
	if (c != '+' && c != '-') || i == 0 {
		return false
	}
	return !t.isNegativeSign(expr, i)
}

// evaluateBinaryOp 计算二元运算。
func (t *CalculatorTool) evaluateBinaryOp(op byte, left, right string) (float64, bool, error) {
	leftVal, err := t.parseExpression(left)
	if err != nil {
		return 0, false, nil
	}

	rightVal, err := t.parseExpression(right)
	if err != nil {
		return 0, true, err
	}

	if op == '+' {
		return leftVal + rightVal, true, nil
	}
	if op == '-' {
		return leftVal - rightVal, true, nil
	}
	if op == '*' {
		return leftVal * rightVal, true, nil
	}
	if op == '/' {
		if rightVal == 0 {
			return 0, true, fmt.Errorf("除数不能为零")
		}
		return leftVal / rightVal, true, nil
	}
	return 0, false, nil
}

// isNegativeSign 判断减号是否为负号。
func (t *CalculatorTool) isNegativeSign(expr string, idx int) bool {
	if idx == 0 {
		return true
	}
	prev := expr[idx-1]
	return prev == '(' || prev == '+' || prev == '-' || prev == '*' || prev == '/'
}

// tryEvaluateMulDiv 尝试处理乘法和除法。
func (t *CalculatorTool) tryEvaluateMulDiv(expr string) (float64, bool, error) {
	for i := len(expr) - 1; i >= 0; i-- {
		c := expr[i]
		if c != '*' && c != '/' {
			continue
		}

		if result, ok, err := t.evaluateBinaryOp(c, expr[:i], expr[i+1:]); ok {
			return result, true, err
		}
	}
	return 0, false, nil
}

// evaluateWithFunctions 计算包含数学函数的表达式。
func (t *CalculatorTool) evaluateWithFunctions(expr string) (float64, error) {
	functions := t.getMathFunctions()

	for _, f := range functions {
		if !strings.Contains(expr, f.name) {
			continue
		}

		start := strings.Index(expr, f.name)
		if start == -1 {
			continue
		}

		end, err := t.findFunctionEnd(expr, start, f.name)
		if err != nil {
			return 0, err
		}

		arg := expr[start+len(f.name) : end]
		argVal, err := t.parseExpression(arg)
		if err != nil {
			return 0, err
		}

		result := f.fn(argVal)
		expr = expr[:start] + fmt.Sprintf("%g", result) + expr[end+1:]
		return t.parseExpression(expr)
	}

	return 0, fmt.Errorf("未知的数学函数")
}

// getMathFunctions 返回支持的数学函数列表。
func (t *CalculatorTool) getMathFunctions() []struct {
	name string
	fn   func(float64) float64
} {
	return []struct {
		name string
		fn   func(float64) float64
	}{
		{name: "sin(", fn: func(x float64) float64 { return t.sin(x * 3.14159265359 / 180) }},
		{name: "cos(", fn: func(x float64) float64 { return t.cos(x * 3.14159265359 / 180) }},
		{name: "tan(", fn: func(x float64) float64 { return t.tan(x * 3.14159265359 / 180) }},
		{name: "sqrt(", fn: func(x float64) float64 { return t.sqrt(x) }},
		{name: "log(", fn: func(x float64) float64 { return t.log10(x) }},
		{name: "ln(", fn: func(x float64) float64 { return t.ln(x) }},
		{name: "abs(", fn: func(x float64) float64 {
			if x < 0 {
				return -x
			}
			return x
		}},
	}
}

// findFunctionEnd 找到函数的结束括号位置。
func (t *CalculatorTool) findFunctionEnd(expr string, start int, funcName string) (int, error) {
	parenCount := 0
	for i := start + len(funcName); i < len(expr); i++ {
		if expr[i] == '(' {
			parenCount++
		} else if expr[i] == ')' {
			if parenCount == 0 {
				return i, nil
			}
			parenCount--
		}
	}
	return -1, fmt.Errorf("函数括号不匹配")
}

// 数学函数实现（简化版本，避免导入 math 包）
func (t *CalculatorTool) pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	if exp == 1 {
		return base
	}
	// 简化实现：仅支持整数指数
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

func (t *CalculatorTool) sqrt(x float64) float64 {
	// 牛顿法求平方根
	if x < 0 {
		return 0
	}
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
		if z*z-x < 1e-10 && z*z-x > -1e-10 {
			break
		}
	}
	return z
}

func (t *CalculatorTool) sin(x float64) float64 {
	// 泰勒级数展开
	result := x
	term := x
	for n := 1; n < 20; n++ {
		term *= -x * x / float64((2*n)*(2*n+1))
		result += term
	}
	return result
}

func (t *CalculatorTool) cos(x float64) float64 {
	// 泰勒级数展开
	result := 1.0
	term := 1.0
	for n := 1; n < 20; n++ {
		term *= -x * x / float64((2*n-1)*(2*n))
		result += term
	}
	return result
}

func (t *CalculatorTool) tan(x float64) float64 {
	return t.sin(x) / t.cos(x)
}

func (t *CalculatorTool) log10(x float64) float64 {
	// 简化实现
	return t.ln(x) / 2.30258509299
}

func (t *CalculatorTool) ln(x float64) float64 {
	// 泰勒级数展开（仅对 x > 0 有效）
	if x <= 0 {
		return 0
	}
	// 使用 ln(x) = 2 * artanh((x-1)/(x+1))
	y := (x - 1) / (x + 1)
	result := 0.0
	yPow := y
	for n := 1; n < 100; n += 2 {
		result += yPow / float64(n)
		yPow *= y * y
	}
	return 2 * result
}

// ===== 搜索工具 =====

// SearchTool 搜索工具。
type SearchTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *SearchTool) Name() string {
	return "search"
}

// Description 返回工具描述。
func (t *SearchTool) Description() string {
	return "网络搜索，支持 Bing 搜索引擎，返回搜索结果"
}

// Run 执行工具。
func (t *SearchTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 30
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供搜索关键词")
	}

	// 优先使用 Bing 搜索
	if apiKey, ok := config["bing_subscription_key"].(string); ok && apiKey != "" {
		return t.bingSearch(query, apiKey, config)
	}

	// 使用 Google 搜索（如果配置了）
	if apiKey, ok := config["google_api_key"].(string); ok && apiKey != "" {
		cseID, _ := config["google_cse_id"].(string)
		return t.googleSearch(query, apiKey, cseID)
	}

	// 如果没有配置搜索引擎，返回提示信息
	return "", fmt.Errorf("未配置搜索引擎 API Key，请在配置文件中设置 bing_subscription_key 或 google_api_key")
}

// bingSearch 使用 Bing 搜索 API。
func (t *SearchTool) bingSearch(query, apiKey string, config map[string]any) (string, error) {
	searchURL := BingSearchURL
	if url, ok := config["bing_search_url"].(string); ok && url != "" {
		searchURL = url
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("%s?q=%s&count=5&mkt=zh-CN", searchURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Ocp-Apim-Subscription-Key", apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var bingResp struct {
		WebPages struct {
			Value []struct {
				Name            string `json:"name"`
				URL             string `json:"url"`
				Snippet         string `json:"snippet"`
				DateLastCrawled string `json:"dateLastCrawled"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &bingResp); err != nil {
		return "", err
	}

	// 格式化结果
	if len(bingResp.WebPages.Value) == 0 {
		return fmt.Sprintf("未找到与 '%s' 相关的搜索结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索: %s\n\n", query))
	sb.WriteString("搜索结果:\n\n")

	for i, result := range bingResp.WebPages.Value {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Name))
		sb.WriteString(fmt.Sprintf("   链接: %s\n", result.URL))
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", result.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// googleSearch 使用 Google Custom Search API。
func (t *SearchTool) googleSearch(query, apiKey, cseID string) (string, error) {
	if cseID == "" {
		return "", fmt.Errorf("未配置 Google Custom Search Engine ID (google_cse_id)")
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=5",
		apiKey, cseID, url.QueryEscape(query))

	resp, err := t.client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var googleResp struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &googleResp); err != nil {
		return "", err
	}

	// 格式化结果
	if len(googleResp.Items) == 0 {
		return fmt.Sprintf("未找到与 '%s' 相关的搜索结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索: %s\n\n", query))
	sb.WriteString("搜索结果:\n\n")

	for i, result := range googleResp.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("   链接: %s\n", result.Link))
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", result.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// ===== 初始化注册 =====

// 初始化时注册内置工具和插件创建器。
func init() {
	RegisterTool(&URLGetTool{})
	RegisterTool(&MeteoTool{})
	RegisterTool(&CalculatorTool{})
	RegisterTool(&SearchTool{})
}
