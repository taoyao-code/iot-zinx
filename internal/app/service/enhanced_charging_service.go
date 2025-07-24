package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// EnhancedChargingService Enhanced版本的充电服务
type EnhancedChargingService struct {
	// DataBus 引用
	dataBus databus.DataBus

	// 核心组件
	responseTracker *CommandResponseTracker

	// 配置
	config *EnhancedChargingConfig

	// 事件订阅管理
	subscriptions map[string]interface{}

	// 充电会话管理
	sessions map[string]*ChargingSession
	mutex    sync.RWMutex

	// 统计信息
	stats *ChargingServiceStats

	// 日志器
	logger *logrus.Logger

	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc
}

// ProcessChargingRequest 处理充电请求
func (s *EnhancedChargingService) ProcessChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("充电请求不能为空")
	}

	// 基本参数验证
	if req.DeviceID == "" {
		return nil, fmt.Errorf("设备ID不能为空")
	}

	if req.Port <= 0 {
		return nil, fmt.Errorf("端口号无效: %d", req.Port)
	}

	// 记录统计
	s.mutex.Lock()
	if s.stats != nil {
		s.stats.TotalRequests++
	}
	s.mutex.Unlock()

	// 根据命令类型处理
	switch req.Command {
	case "start":
		return s.processStartChargingRequest(req)
	case "stop":
		return s.processStopChargingRequest(req)
	case "query":
		return s.processQueryChargingRequest(req)
	default:
		return nil, fmt.Errorf("不支持的充电命令: %s", req.Command)
	}
}

// processStartChargingRequest 处理开始充电请求
func (s *EnhancedChargingService) processStartChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"deviceId":    req.DeviceID,
		"port":        req.Port,
		"orderNumber": req.OrderNumber,
	}).Info("处理开始充电请求")

	// 创建充电会话
	session := &ChargingSession{
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "starting",
		StartTime:   time.Now(),
		Duration:    req.Duration,
		Balance:     req.Balance,
		LastUpdate:  time.Now(),
	}

	// 保存会话
	s.mutex.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*ChargingSession)
	}
	s.sessions[req.OrderNumber] = session
	s.mutex.Unlock()

	// 通过DataBus发布充电开始事件
	if s.dataBus != nil {
		portData := &databus.PortData{
			DeviceID:   req.DeviceID,
			PortNumber: req.Port,
			Status:     "charging",
			IsCharging: true,
			OrderID:    req.OrderNumber,
			LastUpdate: time.Now(),
		}

		if err := s.dataBus.PublishPortData(context.Background(), req.DeviceID, req.Port, portData); err != nil {
			s.logger.WithError(err).Error("发布充电开始数据失败")
		}
	}

	return &ChargingResponse{
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "started",
		Message:     "充电开始成功",
		Timestamp:   time.Now(),
	}, nil
}

// processStopChargingRequest 处理停止充电请求
func (s *EnhancedChargingService) processStopChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"deviceId":    req.DeviceID,
		"port":        req.Port,
		"orderNumber": req.OrderNumber,
	}).Info("处理停止充电请求")

	// 查找并更新会话
	s.mutex.Lock()
	if session, exists := s.sessions[req.OrderNumber]; exists {
		session.Status = "stopped"
		session.LastUpdate = time.Now()
	}
	s.mutex.Unlock()

	// 通过DataBus发布充电停止事件
	if s.dataBus != nil {
		portData := &databus.PortData{
			DeviceID:   req.DeviceID,
			PortNumber: req.Port,
			Status:     "stopped",
			IsCharging: false,
			OrderID:    req.OrderNumber,
			LastUpdate: time.Now(),
		}

		if err := s.dataBus.PublishPortData(context.Background(), req.DeviceID, req.Port, portData); err != nil {
			s.logger.WithError(err).Error("发布充电停止数据失败")
		}
	}

	return &ChargingResponse{
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "stopped",
		Message:     "充电停止成功",
		Timestamp:   time.Now(),
	}, nil
}

// processQueryChargingRequest 处理查询充电请求
func (s *EnhancedChargingService) processQueryChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	// 查询会话状态
	s.mutex.RLock()
	session, exists := s.sessions[req.OrderNumber]
	s.mutex.RUnlock()

	status := "unknown"
	message := "查询成功"

	if exists {
		status = session.Status
	} else {
		message = "未找到充电会话"
	}

	return &ChargingResponse{
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      status,
		Message:     message,
		Timestamp:   time.Now(),
	}, nil
}

// Start 启动Enhanced充电服务
func (s *EnhancedChargingService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.logger.Info("启动Enhanced充电服务")
	return nil
}

// Stop 停止Enhanced充电服务
func (s *EnhancedChargingService) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.logger.Info("停止Enhanced充电服务")
	return nil
}
