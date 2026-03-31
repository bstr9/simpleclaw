package channel

import (
	"context"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/types"
)

func TestChannelType(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Web渠道", ChannelWeb, "web"},
		{"终端渠道", ChannelTerminal, "terminal"},
		{"飞书渠道", ChannelFeishu, "feishu"},
		{"钉钉渠道", ChannelDingtalk, "dingtalk"},
		{"微信渠道", ChannelWeixin, "weixin"},
		{"微信别名", ChannelWeixinAlias, "wx"},
		{"微信公众号", ChannelWechatMP, "wechatmp"},
		{"公众号服务", ChannelWechatMPService, "wechatmp_service"},
		{"企业微信应用", ChannelWechatComApp, "wechatcom_app"},
		{"企业微信机器人", ChannelWecomBot, "wecom_bot"},
		{"QQ渠道", ChannelQQ, "qq"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Channel constant = %s, want %s", tt.constant, tt.expected)
			}
		})
	}
}

func TestBaseChannel_NewBaseChannel(t *testing.T) {
	tests := []struct {
		name        string
		channelType string
	}{
		{"Web渠道", "web"},
		{"终端渠道", "terminal"},
		{"飞书渠道", "feishu"},
		{"空类型", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBaseChannel(tt.channelType)
			if bc == nil {
				t.Fatal("NewBaseChannel returned nil")
			}
			if bc.ChannelType() != tt.channelType {
				t.Errorf("ChannelType() = %s, want %s", bc.ChannelType(), tt.channelType)
			}
			if len(bc.NotSupportTypes()) == 0 {
				t.Error("NotSupportTypes() should have default values")
			}
		})
	}
}

func TestBaseChannel_Name(t *testing.T) {
	bc := NewBaseChannel("test")

	tests := []struct {
		name     string
		setName  string
		expected string
	}{
		{"设置空名称", "", ""},
		{"设置普通名称", "测试用户", "测试用户"},
		{"设置英文名称", "TestUser", "TestUser"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc.SetName(tt.setName)
			if got := bc.Name(); got != tt.expected {
				t.Errorf("Name() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestBaseChannel_UserID(t *testing.T) {
	bc := NewBaseChannel("test")

	tests := []struct {
		name     string
		setID    string
		expected string
	}{
		{"设置空ID", "", ""},
		{"设置数字ID", "12345", "12345"},
		{"设置UUID", "user-uuid-123", "user-uuid-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc.SetUserID(tt.setID)
			if got := bc.UserID(); got != tt.expected {
				t.Errorf("UserID() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestBaseChannel_Started(t *testing.T) {
	bc := NewBaseChannel("test")

	if bc.IsStarted() {
		t.Error("New channel should not be started")
	}

	bc.SetStarted(true)
	if !bc.IsStarted() {
		t.Error("Channel should be started after SetStarted(true)")
	}

	bc.SetStarted(false)
	if bc.IsStarted() {
		t.Error("Channel should not be started after SetStarted(false)")
	}
}

func TestBaseChannel_StartupReport(t *testing.T) {
	t.Run("启动成功", func(t *testing.T) {
		bc := NewBaseChannel("test")
		bc.ReportStartupSuccess()

		if !bc.IsStarted() {
			t.Error("Channel should be started after ReportStartupSuccess")
		}
		if bc.StartupError() != nil {
			t.Errorf("StartupError() should be nil, got %v", bc.StartupError())
		}
	})

	t.Run("启动失败", func(t *testing.T) {
		bc := NewBaseChannel("test")
		testErr := context.Canceled
		bc.ReportStartupError(testErr)

		if bc.IsStarted() {
			t.Error("Channel should not be started after ReportStartupError")
		}
		if bc.StartupError() != testErr {
			t.Errorf("StartupError() = %v, want %v", bc.StartupError(), testErr)
		}
	})
}

func TestBaseChannel_CloudMode(t *testing.T) {
	bc := NewBaseChannel("test")

	if bc.IsCloudMode() {
		t.Error("New channel should not be in cloud mode")
	}

	bc.SetCloudMode(true)
	if !bc.IsCloudMode() {
		t.Error("Channel should be in cloud mode after SetCloudMode(true)")
	}

	bc.SetCloudMode(false)
	if bc.IsCloudMode() {
		t.Error("Channel should not be in cloud mode after SetCloudMode(false)")
	}
}

func TestBaseChannel_ReplyTypeSupport(t *testing.T) {
	bc := NewBaseChannel("test")

	tests := []struct {
		name      string
		replyType types.ReplyType
		supported bool
	}{
		{"文本类型", types.ReplyText, true},
		{"语音类型", types.ReplyVoice, false},
		{"图片类型", types.ReplyImage, false},
		{"图片URL", types.ReplyImageURL, true},
		{"视频URL", types.ReplyVideoURL, true},
		{"文件类型", types.ReplyFile, true},
		{"信息类型", types.ReplyInfo, true},
		{"错误类型", types.ReplyError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bc.IsReplyTypeSupported(tt.replyType); got != tt.supported {
				t.Errorf("IsReplyTypeSupported(%v) = %v, want %v", tt.replyType, got, tt.supported)
			}
		})
	}
}

func TestBaseChannel_SetNotSupportTypes(t *testing.T) {
	bc := NewBaseChannel("test")

	customTypes := []types.ReplyType{types.ReplyText, types.ReplyFile}
	bc.SetNotSupportTypes(customTypes)

	got := bc.NotSupportTypes()
	if len(got) != len(customTypes) {
		t.Errorf("NotSupportTypes() length = %d, want %d", len(got), len(customTypes))
	}

	if bc.IsReplyTypeSupported(types.ReplyText) {
		t.Error("ReplyText should not be supported after SetNotSupportTypes")
	}
	if bc.IsReplyTypeSupported(types.ReplyFile) {
		t.Error("ReplyFile should not be supported after SetNotSupportTypes")
	}
	if !bc.IsReplyTypeSupported(types.ReplyVoice) {
		t.Error("ReplyVoice should be supported after SetNotSupportTypes")
	}
}

func TestBaseChannel_Stop(t *testing.T) {
	bc := NewBaseChannel("test")
	bc.SetStarted(true)

	err := bc.Stop()
	if err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	if bc.IsStarted() {
		t.Error("Channel should not be started after Stop()")
	}
}

func TestBaseChannel_Send(t *testing.T) {
	bc := NewBaseChannel("test")
	reply := types.NewTextReply("test message")
	ctx := types.NewContext(types.ContextText, "hello")

	err := bc.Send(reply, ctx)
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}
}

func TestBaseChannel_Startup(t *testing.T) {
	bc := NewBaseChannel("test")
	ctx := context.Background()

	err := bc.Startup(ctx)
	if err != nil {
		t.Errorf("Startup() returned error: %v", err)
	}
}

func TestBaseChannel_Concurrent(t *testing.T) {
	bc := NewBaseChannel("test")
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			bc.SetName("name1")
			bc.SetUserID("id1")
			bc.SetStarted(true)
			bc.SetCloudMode(true)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = bc.Name()
			_ = bc.UserID()
			_ = bc.IsStarted()
			_ = bc.IsCloudMode()
		}
		done <- true
	}()

	<-done
	<-done
}

type mockChannel struct {
	*BaseChannel
	startupErr error
	stopErr    error
	sendErr    error
}

func newMockChannel(channelType string) *mockChannel {
	return &mockChannel{
		BaseChannel: NewBaseChannel(channelType),
	}
}

func (m *mockChannel) Startup(ctx context.Context) error {
	if m.startupErr != nil {
		return m.startupErr
	}
	m.ReportStartupSuccess()
	return nil
}

func (m *mockChannel) Stop() error {
	m.SetStarted(false)
	return m.stopErr
}

func (m *mockChannel) Send(reply *types.Reply, ctx *types.Context) error {
	return m.sendErr
}

func TestMockChannel(t *testing.T) {
	mc := newMockChannel("mock")

	t.Run("启动成功", func(t *testing.T) {
		err := mc.Startup(context.Background())
		if err != nil {
			t.Errorf("Startup() error = %v", err)
		}
		if !mc.IsStarted() {
			t.Error("MockChannel should be started")
		}
	})

	t.Run("停止成功", func(t *testing.T) {
		err := mc.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}
		if mc.IsStarted() {
			t.Error("MockChannel should be stopped")
		}
	})

	t.Run("发送消息", func(t *testing.T) {
		reply := types.NewTextReply("test")
		ctx := types.NewContext(types.ContextText, "hello")
		err := mc.Send(reply, ctx)
		if err != nil {
			t.Errorf("Send() error = %v", err)
		}
	})
}
