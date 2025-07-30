package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// EnhancedChargingService Enhanced版本的充电服务
type EnhancedChargingService struct {
	// DataBus 引用
	dataBus databus.DataBus

	// 核心组件
	responseTracker *CommandResponseTracker
	sessionManager  session.ISessionManager

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

	// 🔧 新增：检查设备连接状态
	if s.sessionManager == nil {
		s.sessionManager = session.GetGlobalSessionManager()
	}

	// 获取设备连接
	deviceSession, exists := s.sessionManager.GetSession(req.DeviceID)
	if !exists {
		s.logger.WithField("deviceId", req.DeviceID).Error("设备未连接")
		return nil, fmt.Errorf("设备 %s 未连接", req.DeviceID)
	}

	conn := deviceSession.GetConnection()
	if conn == nil {
		s.logger.WithField("deviceId", req.DeviceID).Error("设备连接为空")
		return nil, fmt.Errorf("设备 %s 连接已断开", req.DeviceID)
	}

	// 🔧 新增：构建并发送TCP充电控制命令
	if err := s.sendChargeControlCommand(conn, req); err != nil {
		s.logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"error":    err.Error(),
		}).Error("发送充电控制命令失败")
		return nil, fmt.Errorf("发送充电控制命令失败: %w", err)
	}

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

	s.logger.WithFields(logrus.Fields{
		"deviceId":    req.DeviceID,
		"port":        req.Port,
		"orderNumber": req.OrderNumber,
	}).Info("充电控制命令已发送，等待设备响应")

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

	// 🔧 修复：发送实际的TCP停止充电命令到设备
	// 检查设备连接状态
	if s.sessionManager == nil {
		s.sessionManager = session.GetGlobalSessionManager()
	}

	// 获取设备连接
	deviceSession, exists := s.sessionManager.GetSession(req.DeviceID)
	if !exists {
		s.logger.WithField("deviceId", req.DeviceID).Error("设备未连接")
		return nil, fmt.Errorf("设备 %s 未连接", req.DeviceID)
	}

	conn := deviceSession.GetConnection()
	if conn == nil {
		s.logger.WithField("deviceId", req.DeviceID).Error("设备连接为空")
		return nil, fmt.Errorf("设备 %s 连接已断开", req.DeviceID)
	}

	// 🔧 修复：发送停止充电控制命令（构建停止命令请求）
	stopReq := &ChargingRequest{
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		Command:     "stop", // 确保是停止动作
		OrderNumber: req.OrderNumber,
		Duration:    0, // 停止充电时长为0
		Balance:     0, // 停止充电余额为0
		Mode:        0, // 停止充电模式为0
	}

	if err := s.sendChargeControlCommand(conn, stopReq); err != nil {
		s.logger.WithError(err).Error("发送停止充电命令失败")
		return nil, fmt.Errorf("发送停止充电命令失败: %v", err)
	}

	// 查找并更新会话
	s.mutex.Lock()
	if session, exists := s.sessions[req.OrderNumber]; exists {
		session.Status = "stopped"
		session.LastUpdate = time.Now()

		// 🔧 修复：清理已完成的会话，防止内存泄漏
		// 会话完成后，延迟清理（给用户时间查询最终状态）
		go func(orderNum string) {
			time.Sleep(5 * time.Minute) // 5分钟后清理
			s.mutex.Lock()
			delete(s.sessions, orderNum)
			s.mutex.Unlock()
			s.logger.WithField("orderNumber", orderNum).Debug("已清理完成的充电会话")
		}(req.OrderNumber)
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

	s.logger.WithFields(logrus.Fields{
		"deviceId":    req.DeviceID,
		"port":        req.Port,
		"orderNumber": req.OrderNumber,
	}).Info("停止充电控制命令已发送，等待设备响应")

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

	// 🔧 修复：启动会话清理goroutine，定期清理过期会话
	go s.cleanupExpiredSessions()

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

// cleanupExpiredSessions 清理过期会话，防止内存泄漏
func (s *EnhancedChargingService) cleanupExpiredSessions() {
	ticker := time.NewTicker(10 * time.Minute) // 每10分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("会话清理goroutine已停止")
			return
		case <-ticker.C:
			s.mutex.Lock()
			now := time.Now()
			expiredCount := 0

			for orderNum, session := range s.sessions {
				// 清理已停止超过2小时的会话
				if session.Status == "stopped" && now.Sub(session.LastUpdate) > 2*time.Hour {
					delete(s.sessions, orderNum)
					expiredCount++
				}

				// 清理异常长时间运行的会话（超过24小时）
				if session.Status == "starting" && now.Sub(session.StartTime) > 24*time.Hour {
					delete(s.sessions, orderNum)
					expiredCount++
				}
			}

			if expiredCount > 0 {
				s.logger.WithField("expired_sessions", expiredCount).Info("已清理过期充电会话")
			}

			s.mutex.Unlock()
		}
	}
}

// sendChargeControlCommand 发送充电控制命令到设备
func (s *EnhancedChargingService) sendChargeControlCommand(conn interface{}, req *ChargingRequest) error {
	// 🔧 修复：根据Command字段确定充电命令码
	var chargeCommand byte
	switch req.Command {
	case "start":
		chargeCommand = 1 // 1=开始充电
	case "stop":
		chargeCommand = 2 // 2=停止充电
	default:
		chargeCommand = 1 // 默认为开始充电
	}

	// 构建充电控制命令包使用现有的BuildChargeControlPacket函数
	packet := dny_protocol.BuildChargeControlPacket(
		0,                // physicalID (留空，由连接层填充)
		0,                // messageID (留空，由连接层填充)
		req.Mode,         // rateMode: 费率模式
		req.Balance,      // balance: 余额
		byte(req.Port-1), // portNumber: 端口号(0-based，API是1-based)
		chargeCommand,    // chargeCommand: 根据req.Command动态设置
		req.Duration,     // chargeDuration: 充电时长/电量
		req.OrderNumber,  // orderNumber: 订单编号
		0,                // maxChargeDuration: 最大充电时长(0=使用设备默认值)
		0,                // maxPower: 过载功率(0=使用设备默认值)
		0,                // qrCodeLight: 二维码灯(0=打开)
	)

	// 发送命令到设备
	if tcpConn, ok := conn.(interface{ Write([]byte) (int, error) }); ok {
		bytesWritten, err := tcpConn.Write(packet)
		if err != nil {
			return fmt.Errorf("发送充电控制命令失败: %w", err)
		}

		s.logger.WithFields(logrus.Fields{
			"deviceId":     req.DeviceID,
			"port":         req.Port,
			"command":      fmt.Sprintf("0x82 (sub_cmd=%d)", chargeCommand),
			"orderNumber":  req.OrderNumber,
			"bytesWritten": bytesWritten,
			"packetSize":   len(packet),
			"chargeAction": req.Command,
		}).Info("充电控制命令发送成功")

		return nil
	}

	return fmt.Errorf("连接类型不支持写入操作")
}
