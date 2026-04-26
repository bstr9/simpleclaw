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

// NewWeixinProviderFromChannel 从微信渠道实例创建配对提供者，自动绑定登录状态和二维码 URL
func NewWeixinProviderFromChannel(getLoginStatus func() string, getQRURL func() string) *WeixinProvider {
	return &WeixinProvider{
		getLoginStatus: getLoginStatus,
		getQRURL:       getQRURL,
	}
}

func (p *WeixinProvider) SetLoginStatusFunc(fn func() string) {
	p.getLoginStatus = fn
}

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

func (p *WeixinProvider) GetLoginStatus() string {
	if p.getLoginStatus != nil {
		return p.getLoginStatus()
	}
	return "unknown"
}

func (p *WeixinProvider) GetCurrentQRURL() string {
	if p.getQRURL != nil {
		return p.getQRURL()
	}
	return ""
}
