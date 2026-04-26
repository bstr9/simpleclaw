package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/pair"
	"go.uber.org/zap"
)

type FeishuProvider struct {
	appID     string
	appSecret string
	cliPath   string

	mu           sync.RWMutex
	pendingCodes map[string]string // userID -> deviceCode
}

func NewFeishuProvider(appID, appSecret string) *FeishuProvider {
	cliPath := "lark-cli"
	if path := config.Get().LarkCLIPath; path != "" {
		cliPath = path
	}
	return &FeishuProvider{
		appID:        appID,
		appSecret:    appSecret,
		cliPath:      cliPath,
		pendingCodes: make(map[string]string),
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

	initCmd := exec.CommandContext(ctx, p.cliPath, "config", "init",
		"--app-id", p.appID,
		"--brand", "feishu",
		"--app-secret-stdin",
	)
	initCmd.Env = os.Environ()
	initCmd.Stdin = strings.NewReader(p.appSecret + "\n")

	if _, err := initCmd.CombinedOutput(); err != nil {
		logger.Warn("lark-cli config init failed, continuing with auth login", zap.Error(err))
	}

	cmd := exec.CommandContext(ctx, p.cliPath, "auth", "login",
		"--no-wait",
		"--recommend",
		"--json",
	)

	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lark-cli auth login failed: %w, output: %s", err, string(output))
	}

	var result struct {
		VerificationURL string `json:"verification_url"`
		UserCode        string `json:"user_code"`
		DeviceCode      string `json:"device_code"`
	}

	if err := json.Unmarshal(output, &result); err == nil && result.VerificationURL != "" {
		p.mu.Lock()
		p.pendingCodes[userID] = result.DeviceCode
		p.mu.Unlock()
		return result.VerificationURL, nil
	}

	re := regexp.MustCompile(`https://[^\s"\}]+`)
	if match := re.FindString(string(output)); match != "" {
		return match, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "http") {
			re := regexp.MustCompile(`https://[^\s"\}]+`)
			if match := re.FindString(line); match != "" {
				return match, nil
			}
		}
	}

	return "", fmt.Errorf("could not extract auth URL from output: %s", string(output))
}

func (p *FeishuProvider) CheckStatus(userID string) (pair.PairStatus, error) {
	p.mu.RLock()
	deviceCode, hasCode := p.pendingCodes[userID]
	p.mu.RUnlock()

	if hasCode && deviceCode != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, p.cliPath, "auth", "login",
			"--device-code", deviceCode,
			"--json",
		)
		cmd.Env = os.Environ()

		output, _ := cmd.CombinedOutput()

		var result struct {
			OK    bool `json:"ok"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(output, &result) == nil && result.OK {
			p.mu.Lock()
			delete(p.pendingCodes, userID)
			p.mu.Unlock()
			logger.Info("[FeishuProvider] User authorized via device code", zap.String("user_id", userID))
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.cliPath, "auth", "status")

	cmd.Env = append(os.Environ(),
		"LARK_APP_ID="+p.appID,
		"LARK_APP_SECRET="+p.appSecret,
	)

	output, err := cmd.Output()
	if err != nil {
		return pair.PairStatus{Paired: false, Status: pair.StatusPendingPair}, nil
	}

	var status struct {
		AppID     string `json:"appId"`
		Brand     string `json:"brand"`
		DefaultAs string `json:"defaultAs"`
		Identity  string `json:"identity"`
		Note      string `json:"note"`
	}

	if err := json.Unmarshal(output, &status); err != nil {
		return pair.PairStatus{Paired: false, Status: pair.StatusPendingPair}, nil
	}

	if status.Identity == "user" || (status.Identity == "bot" && status.Note == "") {
		userInfo, _ := p.getUserInfo()
		return pair.PairStatus{
			Paired: true,
			Status: pair.StatusActive,
			Name:   userInfo.Name,
			OpenID: userInfo.OpenID,
		}, nil
	}

	return pair.PairStatus{Paired: false, Status: pair.StatusPendingPair}, nil
}

type feishuUserInfo struct {
	Name   string
	OpenID string
}

func (p *FeishuProvider) getUserInfo() (*feishuUserInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.cliPath, "contact", "+get-user", "--format", "json")
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lark-cli contact +get-user failed: %w", err)
	}

	var result struct {
		OK       bool   `json:"ok"`
		Identity string `json:"identity"`
		Data     struct {
			User struct {
				Name   string `json:"name"`
				OpenID string `json:"open_id"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("get user info failed")
	}

	return &feishuUserInfo{
		Name:   result.Data.User.Name,
		OpenID: result.Data.User.OpenID,
	}, nil
}

func (p *FeishuProvider) IsUserAuthorized(userID string) (bool, error) {
	status, err := p.CheckStatus(userID)
	if err != nil {
		return false, err
	}
	return status.Paired, nil
}
