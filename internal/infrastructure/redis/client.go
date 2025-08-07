package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// 全局Redis客户端实例
var redisClient *redis.Client

// GetClient 获取Redis客户端实例
func GetClient() *redis.Client {
	return redisClient
}

// InitClient 初始化Redis连接
func InitClient() error {
	redisConfig := config.GetConfig().Redis

	// 🔧 修复：如果Redis地址为空，跳过初始化
	if redisConfig.Address == "" {
		logger.Info("Redis配置为空，跳过Redis初始化")
		return nil
	}

	// 创建Redis客户端
	redisClient = redis.NewClient(&redis.Options{
		Addr:         redisConfig.Address,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     redisConfig.PoolSize,
		MinIdleConns: redisConfig.MinIdleConns,
		DialTimeout:  time.Duration(redisConfig.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(redisConfig.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(redisConfig.WriteTimeout) * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		// 🔧 修复：连接失败时清理客户端，避免后续使用空指针
		redisClient = nil
		return fmt.Errorf("Redis连接测试失败: %v", err)
	}

	// 将Redis客户端存储在服务管理器中，以便全局访问
	app.GetServiceManager().SetRedisClient(redisClient)

	logger.Info("Redis连接初始化成功")
	return nil
}

// Close 关闭Redis连接
func Close() error {
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			return fmt.Errorf("关闭Redis连接失败: %v", err)
		}
		logger.Info("Redis连接已关闭")
	}
	return nil
}
