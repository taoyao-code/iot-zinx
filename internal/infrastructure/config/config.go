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
}

// TCPServerConfig TCP服务器配置
type TCPServerConfig struct {
	Host                       string     `mapstructure:"host" yaml:"host"`
	Port                       int        `mapstructure:"port" yaml:"port"`
	Zinx                       ZinxConfig `mapstructure:"zinx" yaml:"zinx"`
	InitialReadDeadlineSeconds int        `mapstructure:"initialReadDeadlineSeconds" yaml:"initialReadDeadlineSeconds"` // 新增：初始读取超时
	DefaultReadDeadlineSeconds int        `mapstructure:"defaultReadDeadlineSeconds" yaml:"defaultReadDeadlineSeconds"` // 新增：默认读取超时
}

// ZinxConfig Zinx框架配置
type ZinxConfig struct {
	Name             string `mapstructure:"name"`
	Version          string `mapstructure:"version"`
	TCPPort          int    `mapstructure:"tcpPort"`
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

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level            string `mapstructure:"level"`
	Format           string `mapstructure:"format"`
	FilePath         string `mapstructure:"filePath"`
	MaxSizeMB        int    `mapstructure:"maxSizeMB"`
	MaxBackups       int    `mapstructure:"maxBackups"`
	MaxAgeDays       int    `mapstructure:"maxAgeDays"`
	LogHexDump       bool   `mapstructure:"logHexDump"`
	EnableConsole    bool   `mapstructure:"enableConsole"`
	EnableStructured bool   `mapstructure:"enableStructured"`
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

// FormatHTTPAddress 格式化HTTP服务器地址为host:port格式
func FormatHTTPAddress() string {
	cfg := GetConfig().HTTPAPIServer
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}
