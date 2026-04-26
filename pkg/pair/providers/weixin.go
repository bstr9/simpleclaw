package providers

import (
	"time"

	"github.com/bstr9/simpleclaw/pkg/pair"
)

type WeixinProvider struct {
	getLoginStatus func() string
	getQRURL       func() string
}

// NewWeixinProvider 创建微信配对提供者
func NewWeixinProvider() *WeixinProvider {
	return &WeixinProvider{}
}

// SetLoginStatusFunc 注入微信登录状态查询函数
// 预留接口：当前 Pair 流程不使用，供未来微信登录状态监控功能调用
func (p *WeixinProvider) SetLoginStatusFunc(fn func() string) {
	p.getLoginStatus = fn
}

// SetQRURLFunc 注入微信二维码 URL 查询函数
// 预留接口：当前 Pair 流程不使用，供未来微信扫码登录引导功能调用
func (p *WeixinProvider) SetQRURLFunc(fn func() string) {
	p.getQRURL = fn
}

func (p *WeixinProvider) ChannelType() string {
	return "weixin"
}

func (p *WeixinProvider) RequiredScopes() []string {
	return []string{}
}

func (p *WeixinProvider) StartPair(userID string) (string, error) {
	return "", nil
}

func (p *WeixinProvider) CheckStatus(userID string) (pair.PairStatus, error) {
	return pair.PairStatus{
		Paired:    true,
		Status:    pair.StatusActive,
		ExpiresAt: time.Time{},
	}, nil
}

func (p *WeixinProvider) IsUserAuthorized(userID string) (bool, error) {
	return true, nil
}

// GetLoginStatus 返回当前微信登录状态字符串
// 预留接口：通过 SetLoginStatusFunc 注入渠道的登录状态查询函数，
// 供未来构建"微信登录状态监控"功能使用。当前 Pair 流程不调用此方法。
func (p *WeixinProvider) GetLoginStatus() string {
	if p.getLoginStatus != nil {
		return p.getLoginStatus()
	}
	return "unknown"
}

// GetCurrentQRURL 返回当前微信登录二维码 URL
// 预留接口：通过 SetQRURLFunc 注入渠道的二维码 URL 查询函数，
// 供未来构建"微信扫码登录引导"功能使用。当前 Pair 流程不调用此方法。
func (p *WeixinProvider) GetCurrentQRURL() string {
	if p.getQRURL != nil {
		return p.getQRURL()
	}
	return ""
}
