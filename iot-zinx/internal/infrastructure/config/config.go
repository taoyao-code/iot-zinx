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
}

// TCPServerConfig TCP服务器配置
type TCPServerConfig struct {
	Host string     `mapstructure:"host"`
	Port int        `mapstructure:"port"`
	Zinx ZinxConfig `mapstructure:"zinx"`
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
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	FilePath   string `mapstructure:"filePath"`
	MaxSizeMB  int    `mapstructure:"maxSizeMB"`
	MaxBackups int    `mapstructure:"maxBackups"`
	MaxAgeDays int    `mapstructure:"maxAgeDays"`
	LogHexDump bool   `mapstructure:"logHexDump"`
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
