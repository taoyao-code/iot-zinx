package notification

import (
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
)

// ConfigAdapter 配置适配器，用于统一不同的配置结构体
// 解决NotificationConfig、NotificationEndpoint、RetryConfig的重复定义问题
type ConfigAdapter struct{}

// NewConfigAdapter 创建配置适配器
func NewConfigAdapter() *ConfigAdapter {
	return &ConfigAdapter{}
}

// ConvertFromInfrastructureConfig 将基础设施配置转换为通知配置
// 统一处理config.NotificationConfig -> notification.NotificationConfig的转换
func (a *ConfigAdapter) ConvertFromInfrastructureConfig(infraConfig *config.NotificationConfig) *NotificationConfig {
	if infraConfig == nil {
		return DefaultNotificationConfig()
	}

	notificationConfig := &NotificationConfig{
		Enabled:   infraConfig.Enabled,
		QueueSize: infraConfig.QueueSize,
		Workers:   infraConfig.Workers,
		Retry:     a.convertRetryConfig(infraConfig.Retry),
	}

	// 转换端点配置
	for _, ep := range infraConfig.Endpoints {
		endpoint := a.convertEndpointConfig(ep)
		notificationConfig.Endpoints = append(notificationConfig.Endpoints, endpoint)
	}

	return notificationConfig
}

// convertEndpointConfig 转换端点配置
// 处理config.NotificationEndpoint -> notification.NotificationEndpoint的转换
func (a *ConfigAdapter) convertEndpointConfig(infraEndpoint config.NotificationEndpoint) NotificationEndpoint {
	return NotificationEndpoint{
		Name:       infraEndpoint.Name,
		Type:       infraEndpoint.Type,
		URL:        infraEndpoint.URL,
		Headers:    infraEndpoint.Headers,
		Timeout:    a.parseDuration(infraEndpoint.Timeout, 10*time.Second),
		EventTypes: infraEndpoint.EventTypes,
		Enabled:    infraEndpoint.Enabled,
	}
}

// convertRetryConfig 转换重试配置
// 处理config.NotificationRetryConfig -> notification.RetryConfig的转换
func (a *ConfigAdapter) convertRetryConfig(infraRetry config.NotificationRetryConfig) RetryConfig {
	return RetryConfig{
		MaxAttempts:     infraRetry.MaxAttempts,
		InitialInterval: a.parseDuration(infraRetry.InitialInterval, 1*time.Second),
		MaxInterval:     a.parseDuration(infraRetry.MaxInterval, 30*time.Second),
		Multiplier:      infraRetry.Multiplier,
	}
}

// parseDuration 解析时间字符串为Duration
// 统一处理字符串时间配置到time.Duration的转换
func (a *ConfigAdapter) parseDuration(durationStr string, defaultValue time.Duration) time.Duration {
	if durationStr == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return defaultValue
	}

	return duration
}

// ConvertToInfrastructureConfig 将通知配置转换为基础设施配置
// 用于反向转换，如果需要的话
func (a *ConfigAdapter) ConvertToInfrastructureConfig(notificationConfig *NotificationConfig) *config.NotificationConfig {
	if notificationConfig == nil {
		return &config.NotificationConfig{}
	}

	infraConfig := &config.NotificationConfig{
		Enabled:   notificationConfig.Enabled,
		QueueSize: notificationConfig.QueueSize,
		Workers:   notificationConfig.Workers,
		Retry:     a.convertToInfraRetryConfig(notificationConfig.Retry),
	}

	// 转换端点配置
	for _, ep := range notificationConfig.Endpoints {
		endpoint := a.convertToInfraEndpointConfig(ep)
		infraConfig.Endpoints = append(infraConfig.Endpoints, endpoint)
	}

	return infraConfig
}

// convertToInfraEndpointConfig 转换端点配置到基础设施格式
func (a *ConfigAdapter) convertToInfraEndpointConfig(endpoint NotificationEndpoint) config.NotificationEndpoint {
	return config.NotificationEndpoint{
		Name:       endpoint.Name,
		Type:       endpoint.Type,
		URL:        endpoint.URL,
		Headers:    endpoint.Headers,
		Timeout:    endpoint.Timeout.String(),
		EventTypes: endpoint.EventTypes,
		Enabled:    endpoint.Enabled,
	}
}

// convertToInfraRetryConfig 转换重试配置到基础设施格式
func (a *ConfigAdapter) convertToInfraRetryConfig(retry RetryConfig) config.NotificationRetryConfig {
	return config.NotificationRetryConfig{
		MaxAttempts:     retry.MaxAttempts,
		InitialInterval: retry.InitialInterval.String(),
		MaxInterval:     retry.MaxInterval.String(),
		Multiplier:      retry.Multiplier,
	}
}

// ValidateConfig 验证配置的有效性
func (a *ConfigAdapter) ValidateConfig(config *NotificationConfig) error {
	if config == nil {
		return nil
	}

	return config.Validate()
}

// MergeConfigs 合并多个配置，后面的配置会覆盖前面的
func (a *ConfigAdapter) MergeConfigs(configs ...*NotificationConfig) *NotificationConfig {
	if len(configs) == 0 {
		return DefaultNotificationConfig()
	}

	result := DefaultNotificationConfig()

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}

		// 合并基本配置
		if cfg.Enabled {
			result.Enabled = cfg.Enabled
		}
		if cfg.QueueSize > 0 {
			result.QueueSize = cfg.QueueSize
		}
		if cfg.Workers > 0 {
			result.Workers = cfg.Workers
		}

		// 合并重试配置
		if cfg.Retry.MaxAttempts > 0 {
			result.Retry.MaxAttempts = cfg.Retry.MaxAttempts
		}
		if cfg.Retry.InitialInterval > 0 {
			result.Retry.InitialInterval = cfg.Retry.InitialInterval
		}
		if cfg.Retry.MaxInterval > 0 {
			result.Retry.MaxInterval = cfg.Retry.MaxInterval
		}
		if cfg.Retry.Multiplier > 0 {
			result.Retry.Multiplier = cfg.Retry.Multiplier
		}

		// 合并端点配置（追加）
		result.Endpoints = append(result.Endpoints, cfg.Endpoints...)
	}

	return result
}

// GetConfigSummary 获取配置摘要信息，用于调试和日志
func (a *ConfigAdapter) GetConfigSummary(config *NotificationConfig) map[string]interface{} {
	if config == nil {
		return map[string]interface{}{
			"enabled": false,
			"error":   "config is nil",
		}
	}

	endpointSummary := make([]map[string]interface{}, len(config.Endpoints))
	for i, ep := range config.Endpoints {
		endpointSummary[i] = map[string]interface{}{
			"name":        ep.Name,
			"type":        ep.Type,
			"enabled":     ep.Enabled,
			"event_count": len(ep.EventTypes),
		}
	}

	return map[string]interface{}{
		"enabled":        config.Enabled,
		"queue_size":     config.QueueSize,
		"workers":        config.Workers,
		"endpoints":      endpointSummary,
		"retry_attempts": config.Retry.MaxAttempts,
	}
}

// 全局配置适配器实例
var DefaultAdapter = NewConfigAdapter()

// 便捷函数，直接使用全局适配器
func ConvertFromInfrastructureConfig(infraConfig *config.NotificationConfig) *NotificationConfig {
	return DefaultAdapter.ConvertFromInfrastructureConfig(infraConfig)
}

func ConvertToInfrastructureConfig(notificationConfig *NotificationConfig) *config.NotificationConfig {
	return DefaultAdapter.ConvertToInfrastructureConfig(notificationConfig)
}

func ValidateNotificationConfig(config *NotificationConfig) error {
	return DefaultAdapter.ValidateConfig(config)
}

func MergeNotificationConfigs(configs ...*NotificationConfig) *NotificationConfig {
	return DefaultAdapter.MergeConfigs(configs...)
}

func GetNotificationConfigSummary(config *NotificationConfig) map[string]interface{} {
	return DefaultAdapter.GetConfigSummary(config)
}
