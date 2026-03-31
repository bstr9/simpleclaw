package channel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

func TestNewChannelManager(t *testing.T) {
	m := NewChannelManager()
	if m == nil {
		t.Fatal("NewChannelManager returned nil")
	}
	if m.ChannelCount() != 0 {
		t.Errorf("New manager should have 0 channels, got %d", m.ChannelCount())
	}
	if m.PrimaryChannel() != nil {
		t.Error("New manager should have nil primary channel")
	}
}

func TestChannelManager_CloudMode(t *testing.T) {
	m := NewChannelManager()

	if m.IsCloudMode() {
		t.Error("New manager should not be in cloud mode")
	}

	m.SetCloudMode(true)
	if !m.IsCloudMode() {
		t.Error("Manager should be in cloud mode after SetCloudMode(true)")
	}

	m.SetCloudMode(false)
	if m.IsCloudMode() {
		t.Error("Manager should not be in cloud mode after SetCloudMode(false)")
	}
}

func TestChannelManager_GetChannel(t *testing.T) {
	m := NewChannelManager()

	ch := m.GetChannel("non_existent")
	if ch != nil {
		t.Error("GetChannel() should return nil for non-existent channel")
	}
}

func TestChannelManager_ListChannels(t *testing.T) {
	m := NewChannelManager()

	list := m.ListChannels()
	if list == nil {
		t.Fatal("ListChannels() returned nil")
	}
	if len(list) != 0 {
		t.Errorf("ListChannels() should return empty slice, got %v", list)
	}
}

func TestChannelManager_Start(t *testing.T) {
	testChannelType := registerTestChannel("manager_start")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if m.ChannelCount() != 1 {
		t.Errorf("ChannelCount() = %d, want 1", m.ChannelCount())
	}

	ch := m.GetChannel(testChannelType)
	if ch == nil {
		t.Fatal("GetChannel() returned nil")
	}

	list := m.ListChannels()
	if len(list) != 1 {
		t.Errorf("ListChannels() length = %d, want 1", len(list))
	}

	m.StopAll()
}

func TestChannelManager_Start_Duplicate(t *testing.T) {
	testChannelType := registerTestChannel("manager_dup")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("First Start() error = %v", err)
	}

	err = m.Start([]string{testChannelType})
	if err == nil {
		t.Error("Start() should return error for duplicate channel")
	}

	m.StopAll()
}

func TestChannelManager_Start_UnknownType(t *testing.T) {
	m := NewChannelManager()
	err := m.Start([]string{"unknown_channel_type_xyz"})
	if err == nil {
		t.Error("Start() should return error for unknown channel type")
	}
}

func TestChannelManager_Stop(t *testing.T) {
	testChannelType := registerTestChannel("manager_stop")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	m.Stop(testChannelType)

	if m.ChannelCount() != 0 {
		t.Errorf("ChannelCount() = %d, want 0 after Stop", m.ChannelCount())
	}

	ch := m.GetChannel(testChannelType)
	if ch != nil {
		t.Error("GetChannel() should return nil after Stop")
	}
}

func TestChannelManager_StopAll(t *testing.T) {
	testChannelType1 := registerTestChannel("manager_stopall1")
	testChannelType2 := registerTestChannel("manager_stopall2")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType1, testChannelType2})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if m.ChannelCount() != 2 {
		t.Errorf("ChannelCount() = %d, want 2", m.ChannelCount())
	}

	m.StopAll()

	if m.ChannelCount() != 0 {
		t.Errorf("ChannelCount() = %d, want 0 after StopAll", m.ChannelCount())
	}
}

func TestChannelManager_PrimaryChannel(t *testing.T) {
	t.Run("非web渠道为主渠道", func(t *testing.T) {
		testChannelType := registerTestChannel("manager_primary")

		m := NewChannelManager()
		err := m.Start([]string{testChannelType})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		pc := m.PrimaryChannel()
		if pc == nil {
			t.Fatal("PrimaryChannel() returned nil")
		}
		if pc.ChannelType() != testChannelType {
			t.Errorf("PrimaryChannel type = %s, want %s", pc.ChannelType(), testChannelType)
		}

		m.StopAll()
	})

	t.Run("web渠道为唯一渠道时为主渠道", func(t *testing.T) {
		webTestType := "web_test_" + time.Now().Format("20060102150405")
		RegisterChannel(webTestType, func() (Channel, error) {
			return newTestManagerChannel(webTestType), nil
		})

		m := NewChannelManager()
		err := m.Start([]string{webTestType})
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		pc := m.PrimaryChannel()
		if pc == nil {
			t.Fatal("PrimaryChannel() should not be nil when web-like is only channel")
		}

		m.StopAll()
	})
}

func TestChannelManager_Restart(t *testing.T) {
	testChannelType := registerTestChannel("manager_restart")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = m.Restart(testChannelType)
	if err != nil {
		t.Fatalf("Restart() error = %v", err)
	}

	if m.ChannelCount() != 1 {
		t.Errorf("ChannelCount() = %d, want 1 after Restart", m.ChannelCount())
	}

	m.StopAll()
}

func TestChannelManager_AddChannel(t *testing.T) {
	testChannelType := registerTestChannel("manager_add")

	m := NewChannelManager()

	err := m.AddChannel(testChannelType)
	if err != nil {
		t.Fatalf("AddChannel() error = %v", err)
	}

	if m.ChannelCount() != 1 {
		t.Errorf("ChannelCount() = %d, want 1", m.ChannelCount())
	}

	m.StopAll()
}

func TestChannelManager_AddChannel_Duplicate(t *testing.T) {
	testChannelType := registerTestChannel("manager_add_dup")

	m := NewChannelManager()

	err := m.AddChannel(testChannelType)
	if err != nil {
		t.Fatalf("First AddChannel() error = %v", err)
	}

	err = m.AddChannel(testChannelType)
	if err != nil {
		t.Fatalf("Second AddChannel() error = %v", err)
	}

	m.StopAll()
}

func TestChannelManager_RemoveChannel(t *testing.T) {
	testChannelType := registerTestChannel("manager_remove")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	m.RemoveChannel(testChannelType)

	if m.ChannelCount() != 0 {
		t.Errorf("ChannelCount() = %d, want 0 after RemoveChannel", m.ChannelCount())
	}
}

func TestChannelManager_RemoveChannel_NonExistent(t *testing.T) {
	m := NewChannelManager()

	m.RemoveChannel("non_existent_channel")
}

func TestChannelManager_Shutdown(t *testing.T) {
	testChannelType := registerTestChannel("manager_shutdown")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	m.Shutdown()

	if m.ChannelCount() != 0 {
		t.Errorf("ChannelCount() = %d, want 0 after Shutdown", m.ChannelCount())
	}
}

func TestChannelManager_Context(t *testing.T) {
	m := NewChannelManager()

	ctx := m.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil")
	}

	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
	}

	m.Shutdown()

	select {
	case <-ctx.Done():
	default:
		t.Error("Context should be done after Shutdown")
	}
}

func TestChannelManager_StartOptions(t *testing.T) {
	testChannelType := registerTestChannel("manager_options")

	tests := []struct {
		name string
		opts []StartOption
	}{
		{"WithFirstStart", []StartOption{WithFirstStart(true)}},
		{"WithInitPlugins", []StartOption{WithInitPlugins(true)}},
		{"MultipleOptions", []StartOption{WithFirstStart(true), WithInitPlugins(true)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewChannelManager()
			err := m.Start([]string{testChannelType}, tt.opts...)
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			m.StopAll()
		})
	}
}

func TestChannelManager_Concurrent(t *testing.T) {
	testChannelType := registerTestChannel("manager_concurrent")

	m := NewChannelManager()
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			m.SetCloudMode(true)
		}()

		go func() {
			defer wg.Done()
			_ = m.IsCloudMode()
		}()

		go func() {
			defer wg.Done()
			_ = m.ChannelCount()
			_ = m.ListChannels()
		}()
	}

	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	wg.Wait()
	m.StopAll()
}

func TestChannelManager_Wait(t *testing.T) {
	testChannelType := registerTestChannel("manager_wait")

	m := NewChannelManager()
	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done := make(chan bool)
	go func() {
		m.Wait()
		done <- true
	}()

	m.Shutdown()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Wait() should return after Shutdown")
	}
}

type testManagerChannel struct {
	*BaseChannel
	startupErr error
}

func newTestManagerChannel(channelType string) *testManagerChannel {
	return &testManagerChannel{
		BaseChannel: NewBaseChannel(channelType),
	}
}

func (m *testManagerChannel) Startup(ctx context.Context) error {
	if m.startupErr != nil {
		return m.startupErr
	}
	m.ReportStartupSuccess()
	return nil
}

func (m *testManagerChannel) Stop() error {
	m.SetStarted(false)
	return nil
}

func (m *testManagerChannel) Send(reply *types.Reply, ctx *types.Context) error {
	return nil
}

func registerTestChannel(baseName string) string {
	channelType := baseName + "_" + time.Now().Format("20060102150405")

	RegisterChannel(channelType, func() (Channel, error) {
		return newTestManagerChannel(channelType), nil
	})

	return channelType
}

func TestOrderChannels(t *testing.T) {
	m := NewChannelManager()

	channels := []struct {
		name    string
		channel Channel
	}{
		{"feishu", newTestManagerChannel("feishu")},
		{"web", newTestManagerChannel("web")},
		{"terminal", newTestManagerChannel("terminal")},
	}

	ordered := m.orderChannels(channels)

	if len(ordered) != 3 {
		t.Fatalf("orderChannels() length = %d, want 3", len(ordered))
	}

	if ordered[0].name != "web" {
		t.Errorf("First channel should be 'web', got '%s'", ordered[0].name)
	}
}

func TestChannelManager_CloudModePropagation(t *testing.T) {
	testChannelType := "manager_cloud_prop_" + time.Now().Format("20060102150405")

	RegisterChannel(testChannelType, func() (Channel, error) {
		return NewBaseChannel(testChannelType), nil
	})

	m := NewChannelManager()
	m.SetCloudMode(true)

	err := m.Start([]string{testChannelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ch := m.GetChannel(testChannelType)
	if bc, ok := ch.(*BaseChannel); ok {
		if !bc.IsCloudMode() {
			t.Error("Channel should inherit cloud mode from manager")
		}
	}

	m.StopAll()
}

func TestChannelManager_StartupError(t *testing.T) {
	channelType := "error_startup_" + time.Now().Format("20060102150405")
	expectedErr := errors.New("startup failed")

	RegisterChannel(channelType, func() (Channel, error) {
		mc := newTestManagerChannel(channelType)
		mc.startupErr = expectedErr
		return mc, nil
	})

	m := NewChannelManager()
	err := m.Start([]string{channelType})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	m.StopAll()
}
