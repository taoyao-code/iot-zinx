package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 是应用程序配置的结构体
type Config struct {
	TCPServer        TCPServerConfig        `mapstructure:"tcpServer"`
	HTTPAPIServer    HTTPAPIServerConfig    `mapstructure:"httpApiServer"`
	Redis            RedisConfig            `mapstructure:"redis"`
	Logger           LoggerConfig           `mapstructure:"logger"`
	BusinessPlatform BusinessPlatformConfig `mapstructure:"businessPlatform"`
	Timeouts         TimeoutsConfig         `mapstructure:"timeouts"`
	DeviceConnection DeviceConnectionConfig `mapstructure:"deviceConnection"`
	Notification     NotificationConfig     `mapstructure:"notification"`
}

// TCPServerConfig TCP服务器配置
type TCPServerConfig struct {
	// 基础配置
	Host string `mapstructure:"host" yaml:"host"`
	Port int    `mapstructure:"port" yaml:"port"`

	// 超时配置
	InitialReadDeadlineSeconds int `mapstructure:"initialReadDeadlineSeconds" yaml:"initialReadDeadlineSeconds"`
	DefaultReadDeadlineSeconds int `mapstructure:"defaultReadDeadlineSeconds" yaml:"defaultReadDeadlineSeconds"`
	TCPWriteTimeoutSeconds     int `mapstructure:"tcpWriteTimeoutSeconds" yaml:"tcpWriteTimeoutSeconds"`
	TCPReadTimeoutSeconds      int `mapstructure:"tcpReadTimeoutSeconds" yaml:"tcpReadTimeoutSeconds"`

	// 缓冲区配置
	SendBufferSize    int `mapstructure:"sendBufferSize" yaml:"sendBufferSize"`
	ReceiveBufferSize int `mapstructure:"receiveBufferSize" yaml:"receiveBufferSize"`

	// TCP选项
	KeepAlive              bool `mapstructure:"keepAlive" yaml:"keepAlive"`
	KeepAlivePeriodSeconds int  `mapstructure:"keepAlivePeriodSeconds" yaml:"keepAlivePeriodSeconds"`
	TCPNoDelay             bool `mapstructure:"tcpNoDelay" yaml:"tcpNoDelay"`

	// 队列配置
	SendQueueSize      int `mapstructure:"sendQueueSize" yaml:"sendQueueSize"`
	ReadQueueSize      int `mapstructure:"readQueueSize" yaml:"readQueueSize"`
	WriteChannelBuffer int `mapstructure:"writeChannelBuffer" yaml:"writeChannelBuffer"`
	ReadChannelBuffer  int `mapstructure:"readChannelBuffer" yaml:"readChannelBuffer"`

	// Zinx框架配置
	Zinx ZinxConfig `mapstructure:"zinx" yaml:"zinx"`
}

// ZinxConfig Zinx框架配置
type ZinxConfig struct {
	Name             string `mapstructure:"name"`
	Version          string `mapstructure:"version"`
	MaxConn          int    `mapstructure:"maxConn"`
	WorkerPoolSize   int    `mapstructure:"workerPoolSize"`
	MaxWorkerTaskLen int    `mapstructure:"maxWorkerTaskLen"`
	MaxPacketSize    uint32 `mapstructure:"maxPacketSize"`
}

// HTTPAPIServerConfig HTTP API服务器配置
type HTTPAPIServerConfig struct {
	Host           string     `mapstructure:"host"`
	Port           int        `mapstructure:"port"`
	Auth           AuthConfig `mapstructure:"auth"`
	TimeoutSeconds int        `mapstructure:"timeoutSeconds"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	SharedKey  string   `mapstructure:"sharedKey"`
	AllowedIPs []string `mapstructure:"allowedIPs"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Address      string `mapstructure:"address"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"poolSize"`
	MinIdleConns int    `mapstructure:"minIdleConns"`
	DialTimeout  int    `mapstructure:"dialTimeout"`
	ReadTimeout  int    `mapstructure:"readTimeout"`
	WriteTimeout int    `mapstructure:"writeTimeout"`
}

// LoggerConfig 统一日志配置
type LoggerConfig struct {
	// 基础配置
	Level            string `mapstructure:"level"`            // 日志级别
	Format           string `mapstructure:"format"`           // 输出格式: json, text
	EnableConsole    bool   `mapstructure:"enableConsole"`    // 是否输出到控制台
	EnableStructured bool   `mapstructure:"enableStructured"` // 是否启用结构化日志
	LogHexDump       bool   `mapstructure:"logHexDump"`       // 是否记录十六进制数据

	// 文件输出配置
	EnableFile bool   `mapstructure:"enableFile"` // 是否输出到文件
	FileDir    string `mapstructure:"fileDir"`    // 日志文件目录
	FilePrefix string `mapstructure:"filePrefix"` // 日志文件前缀

	// 轮转配置
	RotationType string `mapstructure:"rotationType"` // 轮转类型: size, daily
	MaxSizeMB    int    `mapstructure:"maxSizeMB"`    // 按大小轮转: 最大文件大小(MB)
	MaxBackups   int    `mapstructure:"maxBackups"`   // 按大小轮转: 最大备份文件数
	MaxAgeDays   int    `mapstructure:"maxAgeDays"`   // 保留天数
	Compress     bool   `mapstructure:"compress"`     // 是否压缩旧文件

	// 兼容性字段 (废弃，但保留以避免配置错误)
	FilePath string `mapstructure:"filePath"` // 废弃: 使用 fileDir + filePrefix
}

// BusinessPlatformConfig 业务平台API配置
type BusinessPlatformConfig struct {
	APIURL              string `mapstructure:"apiUrl"`
	APIKey              string `mapstructure:"apiKey"`
	TimeoutSeconds      int    `mapstructure:"timeoutSeconds"`
	RetryCount          int    `mapstructure:"retryCount"`
	RetryBaseIntervalMs int    `mapstructure:"retryBaseIntervalMs"`
}

// TimeoutsConfig 超时配置
type TimeoutsConfig struct {
	DeviceInitSeconds            int `mapstructure:"deviceInitSeconds"`
	DnyResponseSeconds           int `mapstructure:"dnyResponseSeconds"`
	HeartbeatIntervalSeconds     int `mapstructure:"heartbeatIntervalSeconds"`
	LinkHeartbeatIntervalSeconds int `mapstructure:"linkHeartbeatIntervalSeconds"`
}

// DeviceConnectionConfig 设备连接配置
type DeviceConnectionConfig struct {
	HeartbeatTimeoutSeconds  int `mapstructure:"heartbeatTimeoutSeconds" yaml:"heartbeatTimeoutSeconds"` // HeartbeatManager 的超时时间
	HeartbeatIntervalSeconds int `mapstructure:"heartbeatIntervalSeconds" yaml:"heartbeatIntervalSeconds"`
	// 生产环境建议设置为 7 分钟 (420 秒)
	HeartbeatWarningThreshold int                    `mapstructure:"heartbeatWarningThreshold" yaml:"heartbeatWarningThreshold"`
	SessionTimeoutMinutes     int                    `mapstructure:"sessionTimeoutMinutes" yaml:"sessionTimeoutMinutes"`
	Timeouts                  DifferentiatedTimeouts `mapstructure:"timeouts" yaml:"timeouts"` // 🔧 新增：差异化超时配置
}

// DifferentiatedTimeouts 差异化超时配置
type DifferentiatedTimeouts struct {
	RegisterTimeoutSeconds          int `mapstructure:"registerTimeoutSeconds" yaml:"registerTimeoutSeconds"`                   // 注册响应超时
	HeartbeatResponseTimeoutSeconds int `mapstructure:"heartbeatResponseTimeoutSeconds" yaml:"heartbeatResponseTimeoutSeconds"` // 心跳响应超时
	DataTransferTimeoutSeconds      int `mapstructure:"dataTransferTimeoutSeconds" yaml:"dataTransferTimeoutSeconds"`           // 数据传输超时
	DefaultWriteTimeoutSeconds      int `mapstructure:"defaultWriteTimeoutSeconds" yaml:"defaultWriteTimeoutSeconds"`           // 默认写操作超时
}

// 全局配置实例
var GlobalConfig Config

// Load 加载配置文件
func Load(configPath string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := v.Unmarshal(&GlobalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return &GlobalConfig
}

// LoadConfig 加载配置文件（向后兼容）
func LoadConfig(configPath string) error {
	return Load(configPath)
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled   bool                    `mapstructure:"enabled"`
	QueueSize int                     `mapstructure:"queue_size"`
	Workers   int                     `mapstructure:"workers"`
	Endpoints []NotificationEndpoint  `mapstructure:"endpoints"`
	Retry     NotificationRetryConfig `mapstructure:"retry"`
}

// NotificationEndpoint 通知端点配置
type NotificationEndpoint struct {
	Name       string            `mapstructure:"name"`
	Type       string            `mapstructure:"type"`
	URL        string            `mapstructure:"url"`
	Headers    map[string]string `mapstructure:"headers"`
	Timeout    string            `mapstructure:"timeout"`
	EventTypes []string          `mapstructure:"event_types"`
	Enabled    bool              `mapstructure:"enabled"`
}

// NotificationRetryConfig 重试配置
type NotificationRetryConfig struct {
	MaxAttempts     int     `mapstructure:"max_attempts"`
	InitialInterval string  `mapstructure:"initial_interval"`
	MaxInterval     string  `mapstructure:"max_interval"`
	Multiplier      float64 `mapstructure:"multiplier"`
}

// FormatHTTPAddress 格式化HTTP服务器地址为host:port格式
func FormatHTTPAddress() string {
	cfg := GetConfig().HTTPAPIServer
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}
