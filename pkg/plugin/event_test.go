package plugin

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestEvent(t *testing.T) {
	t.Run("EventValues", func(t *testing.T) {
		tests := []struct {
			event    Event
			expected int
		}{
			{EventOnReceiveMessage, 1},
			{EventOnHandleContext, 2},
			{EventOnDecorateReply, 3},
			{EventOnSendReply, 4},
		}

		for _, tt := range tests {
			if int(tt.event) != tt.expected {
				t.Errorf("Event value = %d, want %d", tt.event, tt.expected)
			}
		}
	})

	t.Run("EventString", func(t *testing.T) {
		tests := []struct {
			event    Event
			expected string
		}{
			{EventOnReceiveMessage, "ON_RECEIVE_MESSAGE"},
			{EventOnHandleContext, "ON_HANDLE_CONTEXT"},
			{EventOnDecorateReply, "ON_DECORATE_REPLY"},
			{EventOnSendReply, "ON_SEND_REPLY"},
			{Event(999), "UNKNOWN(999)"},
		}

		for _, tt := range tests {
			if got := tt.event.String(); got != tt.expected {
				t.Errorf("Event(%d).String() = %s, want %s", tt.event, got, tt.expected)
			}
		}
	})
}

func TestEventAction(t *testing.T) {
	t.Run("EventActionValues", func(t *testing.T) {
		tests := []struct {
			action   EventAction
			expected int
		}{
			{ActionContinue, 1},
			{ActionBreak, 2},
			{ActionBreakPass, 3},
		}

		for _, tt := range tests {
			if int(tt.action) != tt.expected {
				t.Errorf("EventAction value = %d, want %d", tt.action, tt.expected)
			}
		}
	})

	t.Run("EventActionString", func(t *testing.T) {
		tests := []struct {
			action   EventAction
			expected string
		}{
			{ActionContinue, "CONTINUE"},
			{ActionBreak, "BREAK"},
			{ActionBreakPass, "BREAK_PASS"},
			{EventAction(999), "UNKNOWN(999)"},
		}

		for _, tt := range tests {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("EventAction(%d).String() = %s, want %s", tt.action, got, tt.expected)
			}
		}
	})
}

func TestEventContext(t *testing.T) {
	t.Run("NewEventContext", func(t *testing.T) {
		tests := []struct {
			name  string
			event Event
			data  map[string]any
		}{
			{"with nil data", EventOnReceiveMessage, nil},
			{"with data", EventOnHandleContext, map[string]any{"key": "value"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ec := NewEventContext(tt.event, tt.data)

				if ec.Event != tt.event {
					t.Errorf("Event = %v, want %v", ec.Event, tt.event)
				}
				if ec.Data == nil {
					t.Error("Data should not be nil")
				}
				if ec.Action() != ActionContinue {
					t.Errorf("Action = %v, want CONTINUE", ec.Action())
				}
			})
		}
	})

	t.Run("GetOperations", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"string": "value",
			"int":    42,
			"int64":  int64(100),
			"float":  float64(3.14),
			"bool":   true,
		})

		tests := []struct {
			name     string
			key      string
			expected any
			exists   bool
		}{
			{"string value", "string", "value", true},
			{"int value", "int", 42, true},
			{"bool value", "bool", true, true},
			{"missing value", "missing", nil, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				val, ok := ec.Get(tt.key)
				if ok != tt.exists {
					t.Errorf("Get(%s) exists = %v, want %v", tt.key, ok, tt.exists)
				}
				if ok && val != tt.expected {
					t.Errorf("Get(%s) = %v, want %v", tt.key, val, tt.expected)
				}
			})
		}
	})

	t.Run("GetString", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"str": "hello",
			"int": 42,
		})

		tests := []struct {
			name     string
			key      string
			expected string
			exists   bool
		}{
			{"string value", "str", "hello", true},
			{"non-string value", "int", "", false},
			{"missing key", "missing", "", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				val, ok := ec.GetString(tt.key)
				if ok != tt.exists {
					t.Errorf("GetString(%s) exists = %v, want %v", tt.key, ok, tt.exists)
				}
				if val != tt.expected {
					t.Errorf("GetString(%s) = %s, want %s", tt.key, val, tt.expected)
				}
			})
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"int":    42,
			"int64":  int64(100),
			"float":  float64(200.5),
			"string": "not a number",
		})

		tests := []struct {
			name     string
			key      string
			expected int
			exists   bool
		}{
			{"int value", "int", 42, true},
			{"int64 value", "int64", 100, true},
			{"float value", "float", 200, true},
			{"string value", "string", 0, false},
			{"missing key", "missing", 0, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				val, ok := ec.GetInt(tt.key)
				if ok != tt.exists {
					t.Errorf("GetInt(%s) exists = %v, want %v", tt.key, ok, tt.exists)
				}
				if val != tt.expected {
					t.Errorf("GetInt(%s) = %d, want %d", tt.key, val, tt.expected)
				}
			})
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"true":   true,
			"false":  false,
			"string": "not a bool",
		})

		tests := []struct {
			name     string
			key      string
			expected bool
			exists   bool
		}{
			{"true value", "true", true, true},
			{"false value", "false", false, true},
			{"string value", "string", false, false},
			{"missing key", "missing", false, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				val, ok := ec.GetBool(tt.key)
				if ok != tt.exists {
					t.Errorf("GetBool(%s) exists = %v, want %v", tt.key, ok, tt.exists)
				}
				if val != tt.expected {
					t.Errorf("GetBool(%s) = %v, want %v", tt.key, val, tt.expected)
				}
			})
		}
	})

	t.Run("SetAndDelete", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, nil)

		ec.Set("key1", "value1")
		if val, ok := ec.Get("key1"); !ok || val != "value1" {
			t.Errorf("Set failed: got %v, want value1", val)
		}

		ec.Set("key1", "value2")
		if val, ok := ec.Get("key1"); !ok || val != "value2" {
			t.Errorf("Update failed: got %v, want value2", val)
		}

		ec.Delete("key1")
		if _, ok := ec.Get("key1"); ok {
			t.Error("Delete failed: key should not exist")
		}

		ec.Delete("nonexistent")
	})

	t.Run("ActionOperations", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, nil)

		if ec.Action() != ActionContinue {
			t.Errorf("Default Action = %v, want CONTINUE", ec.Action())
		}

		ec.SetAction(ActionBreak)
		if ec.Action() != ActionBreak {
			t.Errorf("Action = %v, want BREAK", ec.Action())
		}

		ec.SetAction(ActionBreakPass)
		if ec.Action() != ActionBreakPass {
			t.Errorf("Action = %v, want BREAK_PASS", ec.Action())
		}
	})

	t.Run("BreakOperations", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, nil)

		ec.Break("test-plugin")
		if !ec.IsBreak() {
			t.Error("IsBreak should be true after Break()")
		}
		if ec.IsPass() {
			t.Error("IsPass should be false after Break()")
		}
		if ec.BreakedBy() != "test-plugin" {
			t.Errorf("BreakedBy = %s, want test-plugin", ec.BreakedBy())
		}

		ec.SetAction(ActionContinue)
		ec.BreakPass("another-plugin")
		if !ec.IsBreak() {
			t.Error("IsBreak should be true after BreakPass()")
		}
		if !ec.IsPass() {
			t.Error("IsPass should be true after BreakPass()")
		}
		if ec.BreakedBy() != "another-plugin" {
			t.Errorf("BreakedBy = %s, want another-plugin", ec.BreakedBy())
		}
	})

	t.Run("Clone", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"key1": "value1",
			"key2": 42,
		})
		ec.Break("test-plugin")

		cloned := ec.Clone()

		if cloned.Event != ec.Event {
			t.Errorf("Cloned Event = %v, want %v", cloned.Event, ec.Event)
		}
		if cloned.Action() != ec.Action() {
			t.Errorf("Cloned Action = %v, want %v", cloned.Action(), ec.Action())
		}
		if cloned.BreakedBy() != ec.BreakedBy() {
			t.Errorf("Cloned BreakedBy = %s, want %s", cloned.BreakedBy(), ec.BreakedBy())
		}

		ec.Set("key1", "modified")
		if val, _ := cloned.Get("key1"); val == "modified" {
			t.Error("Clone should not share data map with original")
		}
	})

	t.Run("String", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, nil)
		ec.Break("test-plugin")

		str := ec.String()
		if str == "" {
			t.Error("String() should not be empty")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"counter": 0,
		})

		done := make(chan bool, 100)

		for i := 0; i < 50; i++ {
			go func() {
				ec.Set("key", "value")
				ec.Delete("key")
				done <- true
			}()
		}

		for i := 0; i < 50; i++ {
			go func() {
				_, _ = ec.Get("counter")
				_ = ec.Action()
				_ = ec.IsBreak()
				done <- true
			}()
		}

		for i := 0; i < 100; i++ {
			<-done
		}
	})
}

func TestEventBus(t *testing.T) {
	t.Run("NewEventBus", func(t *testing.T) {
		bus := NewEventBus()
		if bus == nil {
			t.Fatal("NewEventBus should not return nil")
		}
		if bus.handlers == nil {
			t.Error("handlers map should be initialized")
		}
	})

	t.Run("SubscribeAndPublish", func(t *testing.T) {
		bus := NewEventBus()
		callCount := 0

		handler := func(ec *EventContext) error {
			callCount++
			return nil
		}

		// Subscribe
		bus.Subscribe(EventOnReceiveMessage, handler)

		ec := NewEventContext(EventOnReceiveMessage, nil)
		err := bus.Publish(EventOnReceiveMessage, ec)
		if err != nil {
			t.Errorf("Publish returned error: %v", err)
		}
		if callCount != 1 {
			t.Errorf("Handler called %d times, want 1", callCount)
		}
	})

	t.Run("MultipleHandlers", func(t *testing.T) {
		bus := NewEventBus()
		results := []string{}

		handler1 := func(ec *EventContext) error {
			results = append(results, "handler1")
			return nil
		}
		handler2 := func(ec *EventContext) error {
			results = append(results, "handler2")
			return nil
		}
		handler3 := func(ec *EventContext) error {
			results = append(results, "handler3")
			return nil
		}

		bus.Subscribe(EventOnHandleContext, handler1)
		bus.Subscribe(EventOnHandleContext, handler2)
		bus.Subscribe(EventOnHandleContext, handler3)

		ec := NewEventContext(EventOnHandleContext, nil)
		err := bus.Publish(EventOnHandleContext, ec)
		if err != nil {
			t.Errorf("Publish returned error: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Got %d handler calls, want 3", len(results))
		}
	})

	t.Run("PublishWithError", func(t *testing.T) {
		bus := NewEventBus()
		expectedErr := errors.New("handler error")

		errorHandler := func(ec *EventContext) error {
			return expectedErr
		}

		bus.Subscribe(EventOnDecorateReply, errorHandler)

		ec := NewEventContext(EventOnDecorateReply, nil)
		err := bus.Publish(EventOnDecorateReply, ec)
		if err != expectedErr {
			t.Errorf("Publish returned %v, want %v", err, expectedErr)
		}
	})

	t.Run("PublishNoSubscribers", func(t *testing.T) {
		bus := NewEventBus()

		ec := NewEventContext(EventOnSendReply, nil)
		err := bus.Publish(EventOnSendReply, ec)
		if err != nil {
			t.Errorf("Publish with no subscribers returned error: %v", err)
		}
	})

	t.Run("MultipleEvents", func(t *testing.T) {
		bus := NewEventBus()
		receiveCount := 0
		sendCount := 0

		receiveHandler := func(ec *EventContext) error {
			receiveCount++
			return nil
		}
		sendHandler := func(ec *EventContext) error {
			sendCount++
			return nil
		}

		bus.Subscribe(EventOnReceiveMessage, receiveHandler)
		bus.Subscribe(EventOnSendReply, sendHandler)

		ec1 := NewEventContext(EventOnReceiveMessage, nil)
		_ = bus.Publish(EventOnReceiveMessage, ec1)

		ec2 := NewEventContext(EventOnSendReply, nil)
		_ = bus.Publish(EventOnSendReply, ec2)

		if receiveCount != 1 {
			t.Errorf("Receive handler called %d times, want 1", receiveCount)
		}
		if sendCount != 1 {
			t.Errorf("Send handler called %d times, want 1", sendCount)
		}
	})

	t.Run("ConcurrentSubscribe", func(t *testing.T) {
		bus := NewEventBus()
		done := make(chan bool, 100)

		for i := 0; i < 100; i++ {
			go func() {
				bus.Subscribe(EventOnReceiveMessage, func(ec *EventContext) error {
					return nil
				})
				done <- true
			}()
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		bus.mu.RLock()
		count := len(bus.handlers[EventOnReceiveMessage])
		bus.mu.RUnlock()

		if count != 100 {
			t.Errorf("Handler count = %d, want 100", count)
		}
	})

	t.Run("ConcurrentPublish", func(t *testing.T) {
		bus := NewEventBus()
		var counter atomic.Int64
		done := make(chan bool, 100)

		bus.Subscribe(EventOnReceiveMessage, func(ec *EventContext) error {
			counter.Add(1)
			return nil
		})

		for i := 0; i < 100; i++ {
			go func() {
				ec := NewEventContext(EventOnReceiveMessage, nil)
				_ = bus.Publish(EventOnReceiveMessage, ec)
				done <- true
			}()
		}

		for i := 0; i < 100; i++ {
			<-done
		}

		if counter.Load() != 100 {
			t.Errorf("Handler called %d times, want 100", counter.Load())
		}
	})
}

func TestPluginEvent(t *testing.T) {
	t.Run("PluginReceivesEvent", func(t *testing.T) {
		p := newMockPlugin("test-plugin", "1.0.0")
		eventReceived := false

		p.RegisterHandler(EventOnReceiveMessage, func(ec *EventContext) error {
			eventReceived = true
			return nil
		})

		ec := NewEventContext(EventOnReceiveMessage, map[string]any{
			"message": "hello",
		})

		err := p.OnEvent(EventOnReceiveMessage, ec)
		if err != nil {
			t.Errorf("OnEvent returned error: %v", err)
		}
		if !eventReceived {
			t.Error("Plugin should have received the event")
		}
	})

	t.Run("PluginBreaksEventChain", func(t *testing.T) {
		p := newMockPlugin("test-plugin", "1.0.0")

		p.RegisterHandler(EventOnHandleContext, func(ec *EventContext) error {
			ec.Break("test-plugin")
			return nil
		})

		ec := NewEventContext(EventOnHandleContext, nil)
		_ = p.OnEvent(EventOnHandleContext, ec)

		if !ec.IsBreak() {
			t.Error("Event should be broken")
		}
		if ec.BreakedBy() != "test-plugin" {
			t.Errorf("BreakedBy = %s, want test-plugin", ec.BreakedBy())
		}
	})

	t.Run("PluginModifiesEventData", func(t *testing.T) {
		p := newMockPlugin("test-plugin", "1.0.0")

		p.RegisterHandler(EventOnDecorateReply, func(ec *EventContext) error {
			ec.Set("modified", true)
			ec.Set("reply", "modified reply")
			return nil
		})

		ec := NewEventContext(EventOnDecorateReply, map[string]any{
			"reply": "original reply",
		})

		_ = p.OnEvent(EventOnDecorateReply, ec)

		if modified, _ := ec.GetBool("modified"); !modified {
			t.Error("modified should be true")
		}
		if reply, _ := ec.GetString("reply"); reply != "modified reply" {
			t.Errorf("reply = %s, want 'modified reply'", reply)
		}
	})

	t.Run("PluginReturnsError", func(t *testing.T) {
		p := newMockPlugin("test-plugin", "1.0.0")
		expectedErr := errors.New("plugin error")

		p.RegisterHandler(EventOnSendReply, func(ec *EventContext) error {
			return expectedErr
		})

		ec := NewEventContext(EventOnSendReply, nil)
		err := p.OnEvent(EventOnSendReply, ec)

		if err != expectedErr {
			t.Errorf("OnEvent returned %v, want %v", err, expectedErr)
		}
	})
}
