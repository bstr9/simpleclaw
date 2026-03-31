package types

import (
	"testing"
	"time"
)

func TestReplyType(t *testing.T) {
	tests := []struct {
		replyType ReplyType
		expected  string
	}{
		{ReplyText, "TEXT"},
		{ReplyImage, "IMAGE"},
		{ReplyImageURL, "IMAGE_URL"},
		{ReplyFile, "FILE"},
		{ReplyVideo, "VIDEO"},
		{ReplyInfo, "INFO"},
		{ReplyError, "ERROR"},
		{ReplyVoice, "VOICE"},
		{ReplyVideoURL, "VIDEO_URL"},
		{ReplyCard, "CARD"},
		{ReplyInviteRoom, "INVITE_ROOM"},
		{ReplyText_, "TEXT_"},
		{ReplyMiniApp, "MINIAPP"},
		{ReplyType(999), "UNKNOWN(999)"},
	}

	for _, tt := range tests {
		if got := tt.replyType.String(); got != tt.expected {
			t.Errorf("ReplyType(%d).String() = %s, want %s", tt.replyType, got, tt.expected)
		}
	}
}

func TestNewTextReply(t *testing.T) {
	reply := NewTextReply("Hello")
	if reply.Type != ReplyText {
		t.Errorf("NewTextReply type = %v, want %v", reply.Type, ReplyText)
	}
	if reply.StringContent() != "Hello" {
		t.Errorf("NewTextReply content = %s, want Hello", reply.StringContent())
	}
}

func TestNewImageReply(t *testing.T) {
	reply := NewImageReply("http://example.com/image.png")
	if reply.Type != ReplyImage {
		t.Errorf("NewImageReply type = %v, want %v", reply.Type, ReplyImage)
	}
	if reply.StringContent() != "http://example.com/image.png" {
		t.Errorf("NewImageReply content = %s, want http://example.com/image.png", reply.StringContent())
	}
}

func TestNewErrorReply(t *testing.T) {
	reply := NewErrorReply("something went wrong")
	if reply.Type != ReplyError {
		t.Errorf("NewErrorReply type = %v, want %v", reply.Type, ReplyError)
	}
	if reply.StringContent() != "something went wrong" {
		t.Errorf("NewErrorReply content = %s, want something went wrong", reply.StringContent())
	}
}

func TestNewInfoReply(t *testing.T) {
	reply := NewInfoReply("info message")
	if reply.Type != ReplyInfo {
		t.Errorf("NewInfoReply type = %v, want %v", reply.Type, ReplyInfo)
	}
}

func TestContext(t *testing.T) {
	ctx := NewContext(ContextText, "test content")
	if ctx.Type != ContextText {
		t.Errorf("Context type = %v, want %v", ctx.Type, ContextText)
	}
	if ctx.Content != "test content" {
		t.Errorf("Context content = %s, want test content", ctx.Content)
	}

	ctx.Set("key", "value")
	if v, ok := ctx.GetString("key"); !ok || v != "value" {
		t.Errorf("Context.Get = %s, want value", v)
	}
}

func TestBaseMessage(t *testing.T) {
	msg := &BaseMessage{
		MsgID:          "123",
		FromUserID:     "user1",
		ToUserID:       "bot",
		Content:        "hello",
		IsGroupMessage: false,
	}

	if msg.GetMsgID() != "123" {
		t.Errorf("GetMsgID = %s, want 123", msg.GetMsgID())
	}
	if msg.GetFromUserID() != "user1" {
		t.Errorf("GetFromUserID = %s, want user1", msg.GetFromUserID())
	}
	if msg.GetContent() != "hello" {
		t.Errorf("GetContent = %s, want hello", msg.GetContent())
	}
}

func TestContextType_String(t *testing.T) {
	tests := []struct {
		ct       ContextType
		expected string
	}{
		{ContextText, "TEXT"},
		{ContextVoice, "VOICE"},
		{ContextImage, "IMAGE"},
		{ContextFile, "FILE"},
		{ContextVideo, "VIDEO"},
		{ContextSharing, "SHARING"},
		{ContextImageCreate, "IMAGE_CREATE"},
		{ContextAcceptFriend, "ACCEPT_FRIEND"},
		{ContextJoinGroup, "JOIN_GROUP"},
		{ContextPatPat, "PATPAT"},
		{ContextFunction, "FUNCTION"},
		{ContextExitGroup, "EXIT_GROUP"},
		{ContextType(999), "UNKNOWN(999)"},
	}

	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.expected {
			t.Errorf("ContextType(%d).String() = %s, want %s", tt.ct, got, tt.expected)
		}
	}
}

func TestContext_Methods(t *testing.T) {
	ctx := NewContext(ContextText, "content")

	t.Run("Get", func(t *testing.T) {
		ctx.Set("key", "value")
		val, ok := ctx.Get("key")
		if !ok || val != "value" {
			t.Errorf("expected value, got %v", val)
		}

		_, ok = ctx.Get("nonexistent")
		if ok {
			t.Error("expected false for nonexistent key")
		}
	})

	t.Run("GetString", func(t *testing.T) {
		ctx.Set("str", "hello")
		val, ok := ctx.GetString("str")
		if !ok || val != "hello" {
			t.Errorf("expected hello, got %s", val)
		}

		ctx.Set("int", 123)
		_, ok = ctx.GetString("int")
		if ok {
			t.Error("expected false for non-string value")
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		ctx.Set("int", 42)
		val, ok := ctx.GetInt("int")
		if !ok || val != 42 {
			t.Errorf("expected 42, got %d", val)
		}

		ctx.Set("int64", int64(100))
		val, ok = ctx.GetInt("int64")
		if !ok || val != 100 {
			t.Errorf("expected 100, got %d", val)
		}

		ctx.Set("float64", float64(200))
		val, ok = ctx.GetInt("float64")
		if !ok || val != 200 {
			t.Errorf("expected 200, got %d", val)
		}

		ctx.Set("str", "not an int")
		_, ok = ctx.GetInt("str")
		if ok {
			t.Error("expected false for non-int value")
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		ctx.Set("bool", true)
		val, ok := ctx.GetBool("bool")
		if !ok || !val {
			t.Errorf("expected true, got %v", val)
		}

		ctx.Set("str", "not a bool")
		_, ok = ctx.GetBool("str")
		if ok {
			t.Error("expected false for non-bool value")
		}
	})

	t.Run("Contains", func(t *testing.T) {
		ctx.Set("exists", "value")
		if !ctx.Contains("exists") {
			t.Error("expected true for existing key")
		}
		if ctx.Contains("nonexistent") {
			t.Error("expected false for nonexistent key")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		ctx.Set("delete_me", "value")
		ctx.Delete("delete_me")
		if ctx.Contains("delete_me") {
			t.Error("expected key to be deleted")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		ctx.Set("key1", "value1")
		ctx.Set("key2", "value2")
		ctx.Clear()
		if len(ctx.Kwargs) != 0 {
			t.Errorf("expected empty Kwargs, got %d items", len(ctx.Kwargs))
		}
	})

	t.Run("Clone", func(t *testing.T) {
		ctx.Set("clone_key", "clone_value")
		cloned := ctx.Clone()
		if cloned.Type != ctx.Type {
			t.Error("cloned type mismatch")
		}
		if cloned.Content != ctx.Content {
			t.Error("cloned content mismatch")
		}
		if cloned.Kwargs["clone_key"] != "clone_value" {
			t.Error("cloned kwargs mismatch")
		}
	})

	t.Run("String", func(t *testing.T) {
		s := ctx.String()
		if s == "" {
			t.Error("expected non-empty string")
		}
	})
}

func TestNewContextWithKwargs(t *testing.T) {
	kwargs := map[string]any{"key": "value"}
	ctx := NewContextWithKwargs(ContextText, "content", kwargs)
	if ctx.Type != ContextText {
		t.Errorf("expected ContextText, got %v", ctx.Type)
	}
	if ctx.Kwargs["key"] != "value" {
		t.Error("kwargs not set correctly")
	}

	ctxNil := NewContextWithKwargs(ContextText, "content", nil)
	if ctxNil.Kwargs == nil {
		t.Error("expected non-nil Kwargs")
	}
}

func TestReply_Methods(t *testing.T) {
	t.Run("IsText", func(t *testing.T) {
		if !NewTextReply("hello").IsText() {
			t.Error("expected true for text reply")
		}
		reply := &Reply{Type: ReplyText_}
		if !reply.IsText() {
			t.Error("expected true for ReplyText_")
		}
		if (&Reply{Type: ReplyImage}).IsText() {
			t.Error("expected false for non-text reply")
		}
	})

	t.Run("IsError", func(t *testing.T) {
		if !NewErrorReply("err").IsError() {
			t.Error("expected true for error reply")
		}
		if NewTextReply("hello").IsError() {
			t.Error("expected false for text reply")
		}
	})

	t.Run("IsInfo", func(t *testing.T) {
		if !NewInfoReply("info").IsInfo() {
			t.Error("expected true for info reply")
		}
		if NewTextReply("hello").IsInfo() {
			t.Error("expected false for text reply")
		}
	})

	t.Run("IsMedia", func(t *testing.T) {
		tests := []struct {
			reply    *Reply
			expected bool
		}{
			{NewImageReply("path"), true},
			{NewImageURLReply("url"), true},
			{NewVoiceReply("path"), true},
			{NewVideoReply("path"), true},
			{NewVideoURLReply("url"), true},
			{NewFileReply("path"), true},
			{NewTextReply("hello"), false},
			{NewErrorReply("err"), false},
		}
		for _, tt := range tests {
			if tt.reply.IsMedia() != tt.expected {
				t.Errorf("IsMedia() for type %v = %v, want %v", tt.reply.Type, tt.reply.IsMedia(), tt.expected)
			}
		}
	})

	t.Run("StringContent with non-string", func(t *testing.T) {
		reply := &Reply{Type: ReplyCard, Content: map[string]string{"key": "value"}}
		content := reply.StringContent()
		if content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("String", func(t *testing.T) {
		reply := NewTextReply("hello")
		s := reply.String()
		if s == "" {
			t.Error("expected non-empty string")
		}
	})
}

func TestReply_Constructors(t *testing.T) {
	t.Run("NewReply", func(t *testing.T) {
		reply := NewReply(ReplyText, "content")
		if reply.Type != ReplyText {
			t.Error("type mismatch")
		}
	})

	t.Run("NewImageURLReply", func(t *testing.T) {
		reply := NewImageURLReply("http://example.com/img.png")
		if reply.Type != ReplyImageURL {
			t.Error("type mismatch")
		}
	})

	t.Run("NewVoiceReply", func(t *testing.T) {
		reply := NewVoiceReply("/path/to/voice.mp3")
		if reply.Type != ReplyVoice {
			t.Error("type mismatch")
		}
	})

	t.Run("NewVideoReply", func(t *testing.T) {
		reply := NewVideoReply("/path/to/video.mp4")
		if reply.Type != ReplyVideo {
			t.Error("type mismatch")
		}
	})

	t.Run("NewVideoURLReply", func(t *testing.T) {
		reply := NewVideoURLReply("http://example.com/video.mp4")
		if reply.Type != ReplyVideoURL {
			t.Error("type mismatch")
		}
	})

	t.Run("NewFileReply", func(t *testing.T) {
		reply := NewFileReply("/path/to/file.pdf")
		if reply.Type != ReplyFile {
			t.Error("type mismatch")
		}
	})

	t.Run("NewCardReply", func(t *testing.T) {
		reply := NewCardReply(map[string]string{"name": "card"})
		if reply.Type != ReplyCard {
			t.Error("type mismatch")
		}
	})

	t.Run("NewInviteRoomReply", func(t *testing.T) {
		reply := NewInviteRoomReply(map[string]string{"room": "id"})
		if reply.Type != ReplyInviteRoom {
			t.Error("type mismatch")
		}
	})

	t.Run("NewMiniAppReply", func(t *testing.T) {
		reply := NewMiniAppReply(map[string]string{"appid": "123"})
		if reply.Type != ReplyMiniApp {
			t.Error("type mismatch")
		}
	})
}

func TestBaseMessage_AllMethods(t *testing.T) {
	now := time.Now()
	msg := &BaseMessage{
		MsgID:          "msg123",
		FromUserID:     "user1",
		ToUserID:       "bot",
		Content:        "hello",
		CreateTime:     now,
		IsGroupMessage: true,
		GroupID:        "group1",
		MsgType:        1,
		Context:        NewContext(ContextText, "hello"),
	}

	t.Run("GetToUserID", func(t *testing.T) {
		if msg.GetToUserID() != "bot" {
			t.Error("ToUserID mismatch")
		}
	})

	t.Run("GetCreateTime", func(t *testing.T) {
		if !msg.GetCreateTime().Equal(now) {
			t.Error("CreateTime mismatch")
		}
	})

	t.Run("IsGroup", func(t *testing.T) {
		if !msg.IsGroup() {
			t.Error("expected true for group message")
		}
	})

	t.Run("GetGroupID", func(t *testing.T) {
		if msg.GetGroupID() != "group1" {
			t.Error("GroupID mismatch")
		}
	})

	t.Run("GetMsgType", func(t *testing.T) {
		if msg.GetMsgType() != 1 {
			t.Error("MsgType mismatch")
		}
	})

	t.Run("GetContext", func(t *testing.T) {
		if msg.GetContext() == nil {
			t.Error("expected non-nil context")
		}
	})

	t.Run("SetContext", func(t *testing.T) {
		newCtx := NewContext(ContextImage, "image content")
		msg.SetContext(newCtx)
		if msg.GetContext() != newCtx {
			t.Error("context not set")
		}
	})
}

func TestMessage_Constructors(t *testing.T) {
	t.Run("NewBaseMessage", func(t *testing.T) {
		msg := NewBaseMessage("id1", "from", "to", "content")
		if msg.MsgID != "id1" {
			t.Error("MsgID mismatch")
		}
		if msg.CreateTime.IsZero() {
			t.Error("CreateTime should be set")
		}
	})

	t.Run("NewGroupMessage", func(t *testing.T) {
		msg := NewGroupMessage("id1", "from", "to", "group1", "content")
		if !msg.IsGroupMessage {
			t.Error("expected group message")
		}
		if msg.GroupID != "group1" {
			t.Error("GroupID mismatch")
		}
	})

	t.Run("NewTextMessage", func(t *testing.T) {
		msg := NewTextMessage("id1", "from", "to", "hello")
		if msg.MsgType != int(ContextText) {
			t.Error("MsgType mismatch")
		}
		if msg.Context == nil {
			t.Error("expected non-nil context")
		}
	})

	t.Run("NewTextMessage with options", func(t *testing.T) {
		customTime := time.Now().Add(-time.Hour)
		customCtx := NewContext(ContextText, "custom")
		msg := NewTextMessage("id1", "from", "to", "hello",
			WithMsgType(2),
			WithCreateTime(customTime),
			WithContext(customCtx),
		)
		if msg.MsgType != 2 {
			t.Error("MsgType option not applied")
		}
		if !msg.CreateTime.Equal(customTime) {
			t.Error("CreateTime option not applied")
		}
		if msg.Context != customCtx {
			t.Error("Context option not applied")
		}
	})

	t.Run("NewGroupTextMessage", func(t *testing.T) {
		msg := NewGroupTextMessage("id1", "from", "to", "group1", "hello")
		if !msg.IsGroupMessage {
			t.Error("expected group message")
		}
		if msg.MsgType != int(ContextText) {
			t.Error("MsgType mismatch")
		}
	})

	t.Run("NewGroupTextMessage with options", func(t *testing.T) {
		msg := NewGroupTextMessage("id1", "from", "to", "group1", "hello",
			WithMsgType(5),
		)
		if msg.MsgType != 5 {
			t.Error("MsgType option not applied")
		}
	})
}

func TestContext_NilKwargs(t *testing.T) {
	ctx := &Context{Type: ContextText, Content: "test"}

	t.Run("Get with nil Kwargs", func(t *testing.T) {
		_, ok := ctx.Get("key")
		if ok {
			t.Error("expected false for nil Kwargs")
		}
	})

	t.Run("Set initializes Kwargs", func(t *testing.T) {
		ctx.Set("key", "value")
		if ctx.Kwargs == nil {
			t.Error("expected Kwargs to be initialized")
		}
	})
}
