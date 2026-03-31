package channel

import (
	"errors"
	"testing"
	"time"
)

func TestRegisterChannel(t *testing.T) {
	testChannelType := "test_register_" + uniqueSuffix()

	RegisterChannel(testChannelType, func() (Channel, error) {
		return newMockChannel(testChannelType), nil
	})

	if !IsChannelRegistered(testChannelType) {
		t.Errorf("Channel '%s' should be registered", testChannelType)
	}

	ch, err := CreateChannel(testChannelType)
	if err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	if ch == nil {
		t.Fatal("CreateChannel() returned nil")
	}
	if ch.ChannelType() != testChannelType {
		t.Errorf("ChannelType() = %s, want %s", ch.ChannelType(), testChannelType)
	}
}

func TestCreateChannel_UnknownType(t *testing.T) {
	_, err := CreateChannel("non_existent_channel_type_xyz")
	if err == nil {
		t.Error("CreateChannel() should return error for unknown type")
	}
}

func TestCreateChannel_CreatorError(t *testing.T) {
	testChannelType := "test_error_" + uniqueSuffix()
	expectedErr := errors.New("creator error")

	RegisterChannel(testChannelType, func() (Channel, error) {
		return nil, expectedErr
	})

	ch, err := CreateChannel(testChannelType)
	if err == nil {
		t.Error("CreateChannel() should return error when creator fails")
	}
	if ch != nil {
		t.Error("CreateChannel() should return nil channel on error")
	}
}

func TestClearChannelCache(t *testing.T) {
	testName := "cache_clear_" + uniqueSuffix()
	ClearChannelCache(testName)
}

func uniqueSuffix() string {
	return time.Now().Format("20060102150405.000000")
}
