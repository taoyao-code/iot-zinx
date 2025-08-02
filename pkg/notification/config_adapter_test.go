package notification

import (
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
)

func TestConfigAdapter_ConvertFromInfrastructureConfig(t *testing.T) {
	adapter := NewConfigAdapter()

	// 测试nil配置
	t.Run("nil config", func(t *testing.T) {
		result := adapter.ConvertFromInfrastructureConfig(nil)
		if result == nil {
			t.Error("Expected default config, got nil")
		}
		// 默认配置是disabled的，这是正确的行为
		if result.Enabled {
			t.Error("Expected default config to be disabled")
		}
	})

	// 测试正常配置转换
	t.Run("normal config", func(t *testing.T) {
		infraConfig := &config.NotificationConfig{
			Enabled:   true,
			QueueSize: 1000,
			Workers:   5,
			Retry: config.NotificationRetryConfig{
				MaxAttempts:     3,
				InitialInterval: "1s",
				MaxInterval:     "30s",
				Multiplier:      2.0,
			},
			Endpoints: []config.NotificationEndpoint{
				{
					Name:       "test-endpoint",
					Type:       "http",
					URL:        "http://example.com/webhook",
					Timeout:    "10s",
					EventTypes: []string{"device_register", "heartbeat"},
					Enabled:    true,
				},
			},
		}

		result := adapter.ConvertFromInfrastructureConfig(infraConfig)

		if !result.Enabled {
			t.Error("Expected enabled to be true")
		}
		if result.QueueSize != 1000 {
			t.Errorf("Expected QueueSize 1000, got %d", result.QueueSize)
		}
		if result.Workers != 5 {
			t.Errorf("Expected Workers 5, got %d", result.Workers)
		}
		if result.Retry.MaxAttempts != 3 {
			t.Errorf("Expected MaxAttempts 3, got %d", result.Retry.MaxAttempts)
		}
		if result.Retry.InitialInterval != time.Second {
			t.Errorf("Expected InitialInterval 1s, got %v", result.Retry.InitialInterval)
		}
		if len(result.Endpoints) != 1 {
			t.Errorf("Expected 1 endpoint, got %d", len(result.Endpoints))
		}
		if result.Endpoints[0].Name != "test-endpoint" {
			t.Errorf("Expected endpoint name 'test-endpoint', got '%s'", result.Endpoints[0].Name)
		}
	})
}

func TestConfigAdapter_ParseDuration(t *testing.T) {
	adapter := NewConfigAdapter()

	tests := []struct {
		name         string
		durationStr  string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "valid duration",
			durationStr:  "5s",
			defaultValue: time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "empty string",
			durationStr:  "",
			defaultValue: 10 * time.Second,
			expected:     10 * time.Second,
		},
		{
			name:         "invalid duration",
			durationStr:  "invalid",
			defaultValue: 15 * time.Second,
			expected:     15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.parseDuration(tt.durationStr, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfigAdapter_ConvertToInfrastructureConfig(t *testing.T) {
	adapter := NewConfigAdapter()

	// 测试nil配置
	t.Run("nil config", func(t *testing.T) {
		result := adapter.ConvertToInfrastructureConfig(nil)
		if result == nil {
			t.Error("Expected empty config, got nil")
		}
	})

	// 测试正常配置转换
	t.Run("normal config", func(t *testing.T) {
		notificationConfig := &NotificationConfig{
			Enabled:   true,
			QueueSize: 1000,
			Workers:   5,
			Retry: RetryConfig{
				MaxAttempts:     3,
				InitialInterval: time.Second,
				MaxInterval:     30 * time.Second,
				Multiplier:      2.0,
			},
			Endpoints: []NotificationEndpoint{
				{
					Name:       "test-endpoint",
					Type:       "http",
					URL:        "http://example.com/webhook",
					Timeout:    10 * time.Second,
					EventTypes: []string{"device_register", "heartbeat"},
					Enabled:    true,
				},
			},
		}

		result := adapter.ConvertToInfrastructureConfig(notificationConfig)

		if !result.Enabled {
			t.Error("Expected enabled to be true")
		}
		if result.QueueSize != 1000 {
			t.Errorf("Expected QueueSize 1000, got %d", result.QueueSize)
		}
		if result.Workers != 5 {
			t.Errorf("Expected Workers 5, got %d", result.Workers)
		}
		if result.Retry.MaxAttempts != 3 {
			t.Errorf("Expected MaxAttempts 3, got %d", result.Retry.MaxAttempts)
		}
		if result.Retry.InitialInterval != "1s" {
			t.Errorf("Expected InitialInterval '1s', got '%s'", result.Retry.InitialInterval)
		}
		if len(result.Endpoints) != 1 {
			t.Errorf("Expected 1 endpoint, got %d", len(result.Endpoints))
		}
		if result.Endpoints[0].Name != "test-endpoint" {
			t.Errorf("Expected endpoint name 'test-endpoint', got '%s'", result.Endpoints[0].Name)
		}
	})
}

func TestConfigAdapter_MergeConfigs(t *testing.T) {
	adapter := NewConfigAdapter()

	// 测试空配置合并
	t.Run("empty configs", func(t *testing.T) {
		result := adapter.MergeConfigs()
		if result == nil {
			t.Error("Expected default config, got nil")
		}
	})

	// 测试配置合并
	t.Run("merge configs", func(t *testing.T) {
		config1 := &NotificationConfig{
			Enabled:   true,
			QueueSize: 500,
			Workers:   3,
		}

		config2 := &NotificationConfig{
			QueueSize: 1000,
			Workers:   5,
			Retry: RetryConfig{
				MaxAttempts: 5,
			},
		}

		result := adapter.MergeConfigs(config1, config2)

		if !result.Enabled {
			t.Error("Expected enabled to be true")
		}
		if result.QueueSize != 1000 {
			t.Errorf("Expected QueueSize 1000 (from config2), got %d", result.QueueSize)
		}
		if result.Workers != 5 {
			t.Errorf("Expected Workers 5 (from config2), got %d", result.Workers)
		}
		if result.Retry.MaxAttempts != 5 {
			t.Errorf("Expected MaxAttempts 5 (from config2), got %d", result.Retry.MaxAttempts)
		}
	})
}

func TestConfigAdapter_GetConfigSummary(t *testing.T) {
	adapter := NewConfigAdapter()

	// 测试nil配置
	t.Run("nil config", func(t *testing.T) {
		summary := adapter.GetConfigSummary(nil)
		if summary["enabled"].(bool) {
			t.Error("Expected enabled to be false for nil config")
		}
		if summary["error"] == nil {
			t.Error("Expected error message for nil config")
		}
	})

	// 测试正常配置
	t.Run("normal config", func(t *testing.T) {
		config := &NotificationConfig{
			Enabled:   true,
			QueueSize: 1000,
			Workers:   5,
			Endpoints: []NotificationEndpoint{
				{
					Name:       "endpoint1",
					Type:       "http",
					Enabled:    true,
					EventTypes: []string{"event1", "event2"},
				},
				{
					Name:       "endpoint2",
					Type:       "webhook",
					Enabled:    false,
					EventTypes: []string{"event3"},
				},
			},
			Retry: RetryConfig{
				MaxAttempts: 3,
			},
		}

		summary := adapter.GetConfigSummary(config)

		if !summary["enabled"].(bool) {
			t.Error("Expected enabled to be true")
		}
		if summary["queue_size"].(int) != 1000 {
			t.Errorf("Expected queue_size 1000, got %v", summary["queue_size"])
		}
		if summary["workers"].(int) != 5 {
			t.Errorf("Expected workers 5, got %v", summary["workers"])
		}
		if summary["retry_attempts"].(int) != 3 {
			t.Errorf("Expected retry_attempts 3, got %v", summary["retry_attempts"])
		}

		endpoints := summary["endpoints"].([]map[string]interface{})
		if len(endpoints) != 2 {
			t.Errorf("Expected 2 endpoints in summary, got %d", len(endpoints))
		}
		if endpoints[0]["name"].(string) != "endpoint1" {
			t.Errorf("Expected first endpoint name 'endpoint1', got '%v'", endpoints[0]["name"])
		}
		if endpoints[0]["event_count"].(int) != 2 {
			t.Errorf("Expected first endpoint event_count 2, got %v", endpoints[0]["event_count"])
		}
	})
}

func TestGlobalFunctions(t *testing.T) {
	// 测试全局便捷函数
	infraConfig := &config.NotificationConfig{
		Enabled:   true,
		QueueSize: 1000,
	}

	// 测试ConvertFromInfrastructureConfig
	result := ConvertFromInfrastructureConfig(infraConfig)
	if !result.Enabled {
		t.Error("Expected enabled to be true")
	}
	if result.QueueSize != 1000 {
		t.Errorf("Expected QueueSize 1000, got %d", result.QueueSize)
	}

	// 测试ConvertToInfrastructureConfig
	backConverted := ConvertToInfrastructureConfig(result)
	if !backConverted.Enabled {
		t.Error("Expected back-converted enabled to be true")
	}
	if backConverted.QueueSize != 1000 {
		t.Errorf("Expected back-converted QueueSize 1000, got %d", backConverted.QueueSize)
	}

	// 测试ValidateNotificationConfig
	err := ValidateNotificationConfig(result)
	if err != nil {
		t.Errorf("Expected no validation error, got %v", err)
	}

	// 测试MergeNotificationConfigs
	config2 := &NotificationConfig{Workers: 10}
	merged := MergeNotificationConfigs(result, config2)
	if merged.Workers != 10 {
		t.Errorf("Expected merged workers 10, got %d", merged.Workers)
	}

	// 测试GetNotificationConfigSummary
	summary := GetNotificationConfigSummary(merged)
	if !summary["enabled"].(bool) {
		t.Error("Expected summary enabled to be true")
	}
}
