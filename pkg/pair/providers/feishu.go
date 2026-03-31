package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/pair"
)

const (
	StatusPendingPair = "pending"
	StatusActive      = "active"
	StatusExpired     = "expired"
)

type FeishuProvider struct {
	appID     string
	appSecret string
}

func NewFeishuProvider(appID, appSecret string) *FeishuProvider {
	return &FeishuProvider{
		appID:     appID,
		appSecret: appSecret,
	}
}

func (p *FeishuProvider) ChannelType() string {
	return "feishu"
}

func (p *FeishuProvider) RequiredScopes() []string {
	return []string{
		"calendar:calendar:read",
		"calendar:calendar.event:read",
		"docs:document:readonly",
		"drive:drive.metadata:readonly",
		"drive:file:download",
	}
}

func (p *FeishuProvider) StartPair(userID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lark-cli", "auth", "login",
		"--no-wait",
		"--recommend",
		"--json",
	)

	if p.appID != "" && p.appSecret != "" {
		cmd.Env = append(cmd.Env,
			"LARK_APP_ID="+p.appID,
			"LARK_APP_SECRET="+p.appSecret,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lark-cli auth login failed: %w, output: %s", err, string(output))
	}

	var result struct {
		VerificationURI string `json:"verification_uri"`
		UserCode        string `json:"user_code"`
		DeviceCode      string `json:"device_code"`
	}

	if err := json.Unmarshal(output, &result); err == nil && result.VerificationURI != "" {
		return result.VerificationURI, nil
	}

	re := regexp.MustCompile(`https://[^\s]+`)
	if match := re.FindString(string(output)); match != "" {
		return match, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "http") {
			re := regexp.MustCompile(`https://[^\s]+`)
			if match := re.FindString(line); match != "" {
				return match, nil
			}
		}
	}

	return "", fmt.Errorf("could not extract auth URL from output: %s", string(output))
}

func (p *FeishuProvider) CheckStatus(userID string) (pair.PairStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lark-cli", "auth", "status", "--json")

	if p.appID != "" && p.appSecret != "" {
		cmd.Env = append(cmd.Env,
			"LARK_APP_ID="+p.appID,
			"LARK_APP_SECRET="+p.appSecret,
		)
	}

	output, err := cmd.Output()
	if err != nil {
		return pair.PairStatus{Paired: false, Status: StatusPendingPair}, nil
	}

	var status struct {
		TokenStatus string `json:"tokenStatus"`
		UserOpenID  string `json:"userOpenId"`
		UserName    string `json:"userName"`
		ExpiresAt   string `json:"expiresAt"`
	}

	if err := json.Unmarshal(output, &status); err != nil {
		return pair.PairStatus{Paired: false, Status: StatusPendingPair}, nil
	}

	if status.TokenStatus == "valid" {
		expiresAt, _ := time.Parse(time.RFC3339, status.ExpiresAt)
		return pair.PairStatus{
			Paired:    true,
			Status:    StatusActive,
			ExpiresAt: expiresAt,
		}, nil
	}

	return pair.PairStatus{Paired: false, Status: StatusPendingPair}, nil
}

func (p *FeishuProvider) IsUserAuthorized(userID string) (bool, error) {
	status, err := p.CheckStatus(userID)
	if err != nil {
		return false, err
	}
	return status.Paired, nil
}
