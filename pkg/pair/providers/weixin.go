package providers

import (
	"time"

	"github.com/bstr9/simpleclaw/pkg/pair"
)

type WeixinProvider struct {
	getLoginStatus func() string
	getQRURL       func() string
}

func NewWeixinProvider() *WeixinProvider {
	return &WeixinProvider{}
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
