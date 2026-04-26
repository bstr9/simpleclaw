// api.go — 微信 ilink bot HTTP API 客户端
// 从 weixin_channel.go 提取的 weixinAPI 结构体及其方法
package weixin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/common"
)

// 以下常量在 weixin_channel.go 中也有定义，
// 待 weixin_channel.go 重构后删除该处的重复定义即可编译通过。
const (
	// 默认 API 端点
	defaultBaseURL    = "https://ilinkai.weixin.qq.com"
	defaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"

	// 超时设置
	defaultLongPollTimeout = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second

	// 消息限制
	textChunkLimit = 4000

	// 消息类型
	messageTypeUser = 1 // 用户消息
	messageTypeBot  = 2 // 机器人消息

	// 消息项类型
	itemText  = 1
	itemImage = 2
	itemVoice = 3
	itemFile  = 4
	itemVideo = 5

	// CDN 媒体类型
	mediaTypeImage = 1
	mediaTypeVideo = 2
	mediaTypeFile  = 3

	// 日志前缀
	logPrefix = "[Weixin]"
)

// weixinAPI 实现 ilink bot HTTP API
type weixinAPI struct {
	baseURL    string
	token      string
	cdnBaseURL string
	client     *http.Client
}

// newWeixinAPI 创建微信 API 客户端
func newWeixinAPI(baseURL, token, cdnBaseURL string, client *http.Client) *weixinAPI {
	if cdnBaseURL == "" {
		cdnBaseURL = defaultCDNBaseURL
	}
	return &weixinAPI{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		cdnBaseURL: cdnBaseURL,
		client:     client,
	}
}

// buildHeaders 构建请求头
func (a *weixinAPI) buildHeaders() map[string]string {
	// 生成随机微信 UIN
	val := make([]byte, 4)
	rand.Read(val)
	uinVal := uint32(val[0])<<24 | uint32(val[1])<<16 | uint32(val[2])<<8 | uint32(val[3])
	uin := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%d", uinVal))

	headers := map[string]string{
		common.HeaderContentType:   common.ContentTypeJSON,
		"AuthorizationType":        "ilink_bot_token",
		"X-WECHAT-UIN":             uin,
		"iLink-App-Id":             "1001",
		"iLink-App-ClientVersion":  "1",
	}
	if a.token != "" {
		headers["Authorization"] = common.AuthPrefixBearer + a.token
	}
	return headers
}

// qrCodeResponse 二维码响应
type qrCodeResponse struct {
	QRCode         string `json:"qrcode"`
	QRImageContent string `json:"qrcode_img_content"`
}

// qrStatusResponse 二维码扫码状态响应
type qrStatusResponse struct {
	Status       string `json:"status"`
	BotToken     string `json:"bot_token"`
	BotID        string `json:"ilink_bot_id"`
	BaseURL      string `json:"baseurl"`
	UserID       string `json:"ilink_user_id"`
	RedirectHost string `json:"redirect_host"` // IDC 重定向主机
}

// updatesResponse 消息更新响应
type updatesResponse struct {
	Ret           int              `json:"ret"`
	ErrCode       int              `json:"errcode"`
	ErrMsg        string           `json:"errmsg"`
	GetUpdatesBuf string           `json:"get_updates_buf"`
	Messages      []map[string]any `json:"msgs"`
	BaseURL       string           `json:"baseurl"` // IDC 重定向 URL
}

// uploadURLResponse 上传地址响应
type uploadURLResponse struct {
	UploadParam string `json:"upload_param"`
}

// getConfigResponse 配置获取响应
type getConfigResponse struct {
	Ret          int    `json:"ret"`
	ErrMsg       string `json:"errmsg"`
	TypingTicket string `json:"typing_ticket"` // typing 票据（Base64 编码）
}

// typingStatus typing 指示器状态
type typingStatus int

const (
	typingStatusTyping typingStatus = 1 // 正在输入
	typingStatusCancel typingStatus = 2 // 取消输入
)

// fetchQRCode 获取登录二维码
func (a *weixinAPI) fetchQRCode(ctx context.Context) (*qrCodeResponse, error) {
	apiURL := fmt.Sprintf("%s/ilink/bot/get_bot_qrcode?bot_type=3", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result qrCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// pollQRStatus 轮询二维码扫码状态
func (a *weixinAPI) pollQRStatus(ctx context.Context, qrcode string) (*qrStatusResponse, error) {
	escaped := url.QueryEscape(qrcode)
	urlStr := fmt.Sprintf("%s/ilink/bot/get_qrcode_status?qrcode=%s", a.baseURL, escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result qrStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// getUpdates 获取消息更新
func (a *weixinAPI) getUpdates(buf string) (*updatesResponse, error) {
	body := map[string]any{"get_updates_buf": buf}
	var result updatesResponse
	if err := a.post("ilink/bot/getupdates", body, &result, int(defaultLongPollTimeout.Seconds())+5); err != nil {
		return nil, err
	}
	return &result, nil
}

// sendText 发送文本消息
func (a *weixinAPI) sendText(to, text, contextToken string) error {
	body := map[string]any{
		"msg": map[string]any{
			"from_user_id":  "",
			"to_user_id":    to,
			"client_id":     generateClientID(),
			"message_type":  messageTypeBot,
			"message_state": 2, // FINISH
			"item_list": []map[string]any{
				{"type": itemText, "text_item": map[string]any{"text": text}},
			},
			"context_token": contextToken,
		},
	}
	return a.post("ilink/bot/sendmessage", body, nil, 0)
}

// sendImageItem 发送图片消息
func (a *weixinAPI) sendImageItem(to, contextToken, encryptParam, aesKeyB64 string, ciphertextSize int) error {
	items := []map[string]any{
		{
			"type": itemImage,
			"image_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"mid_size": ciphertextSize,
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

// sendFileItem 发送文件消息
func (a *weixinAPI) sendFileItem(to, contextToken, encryptParam, aesKeyB64, fileName string, fileSize int) error {
	items := []map[string]any{
		{
			"type": itemFile,
			"file_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"file_name": fileName,
				"len":       fmt.Sprintf("%d", fileSize),
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

// sendVideoItem 发送视频消息
func (a *weixinAPI) sendVideoItem(to, contextToken, encryptParam, aesKeyB64 string, ciphertextSize int) error {
	items := []map[string]any{
		{
			"type": itemVideo,
			"video_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"video_size": ciphertextSize,
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

// sendItems 发送消息项列表
func (a *weixinAPI) sendItems(to, contextToken string, items []map[string]any) error {
	body := map[string]any{
		"msg": map[string]any{
			"from_user_id":  "",
			"to_user_id":    to,
			"client_id":     generateClientID(),
			"message_type":  messageTypeBot,
			"message_state": 2,
			"item_list":     items,
			"context_token": contextToken,
		},
	}
	return a.post("ilink/bot/sendmessage", body, nil, 0)
}

// getUploadURL 获取文件上传地址
func (a *weixinAPI) getUploadURL(fileKey string, mediaType int, toUserID string, rawSize int, rawMD5 string, fileSize int, aesKey string) (*uploadURLResponse, error) {
	body := map[string]any{
		"filekey":       fileKey,
		"media_type":    mediaType,
		"to_user_id":    toUserID,
		"rawsize":       rawSize,
		"rawfilemd5":    rawMD5,
		"filesize":      fileSize,
		"aeskey":        aesKey,
		"no_need_thumb": true,
	}
	var result uploadURLResponse
	if err := a.post("ilink/bot/getuploadurl", body, &result, 0); err != nil {
		return nil, err
	}
	return &result, nil
}

// getConfig 获取用户配置（包含 typing_ticket）
func (a *weixinAPI) getConfig(ilinkUserID, contextToken string) (typingTicket string, err error) {
	body := map[string]any{
		"ilink_user_id": ilinkUserID,
		"context_token": contextToken,
	}
	var result getConfigResponse
	if err := a.post("ilink/bot/getconfig", body, &result, 10); err != nil {
		return "", err
	}
	return result.TypingTicket, nil
}

// sendTyping 发送/取消输入状态指示器
// status: 1=正在输入, 2=取消输入
func (a *weixinAPI) sendTyping(ilinkUserID, typingTicket string, status typingStatus) error {
	if typingTicket == "" {
		return nil // 无票据则跳过
	}
	body := map[string]any{
		"ilink_user_id": ilinkUserID,
		"typing_ticket": typingTicket,
		"status":        int(status),
	}
	return a.post("ilink/bot/sendtyping", body, nil, 10)
}

// post 发送 POST 请求
func (a *weixinAPI) post(endpoint string, body any, result any, timeoutSec int) error {
	apiURL := a.baseURL + "/" + endpoint

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	for k, v := range a.buildHeaders() {
		req.Header.Set(k, v)
	}

	client := a.client
	if timeoutSec > 0 {
		client = &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

// updateBaseURL 更新 API 基础 URL（IDC 重定向时调用）
func (a *weixinAPI) updateBaseURL(newURL string) {
	if newURL != "" {
		a.baseURL = strings.TrimSuffix(newURL, "/")
	}
}

// generateClientID 生成客户端 ID
func generateClientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
