package gateway

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// DeviceGateway IoT设备网关统一接口
// 提供简洁、直观的设备管理API，隐藏底层复杂实现
type DeviceGateway struct {
	tcpManager *core.TCPManager
	tcpWriter  *network.TCPWriter // 🚀 Phase 2: 添加TCPWriter支持重试机制
	// AP3000 节流：同设备命令间隔≥0.5秒
	throttleMu       sync.Mutex
	lastSendByDevice map[string]time.Time

	// 订单上下文缓存：deviceID|protocolPort(0-based) → ctx
	orderCtxMu sync.RWMutex
	orderCtx   map[string]OrderContext
}

// NewDeviceGateway 创建设备网关实例
func NewDeviceGateway() *DeviceGateway {
	// 🔧 修复：从配置创建TCPWriter，设置正确的写超时时间
	retryConfig := network.DefaultRetryConfig

	// 尝试从全局配置获取TCP写超时配置
	if globalConfig := config.GetConfig(); globalConfig != nil {
		if globalConfig.TCPServer.TCPWriteTimeoutSeconds > 0 {
			retryConfig.WriteTimeout = time.Duration(globalConfig.TCPServer.TCPWriteTimeoutSeconds) * time.Second
			logger.GetLogger().WithFields(logrus.Fields{
				"writeTimeoutSeconds": globalConfig.TCPServer.TCPWriteTimeoutSeconds,
				"writeTimeout":        retryConfig.WriteTimeout,
			}).Info("✅ TCP写入超时配置已从配置文件加载")
		}
	}

	return &DeviceGateway{
		tcpManager:       core.GetGlobalTCPManager(),
		tcpWriter:        network.NewTCPWriter(retryConfig, logger.GetLogger()),
		lastSendByDevice: make(map[string]time.Time),
		orderCtx:         make(map[string]OrderContext),
	}
}

// ===============================
// 全局网关实例管理
// ===============================

var globalDeviceGateway *DeviceGateway

// GetGlobalDeviceGateway 获取全局设备网关实例
func GetGlobalDeviceGateway() *DeviceGateway {
	if globalDeviceGateway == nil {
		globalDeviceGateway = NewDeviceGateway()
		logger.Info("全局设备网关已初始化")
	}
	return globalDeviceGateway
}

// InitializeGlobalDeviceGateway 初始化全局设备网关
func InitializeGlobalDeviceGateway() {
	globalDeviceGateway = NewDeviceGateway()
	logger.Info("全局设备网关初始化完成")
}
