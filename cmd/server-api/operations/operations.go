package operations

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/client"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/models"
)

// OperationManager 管理API操作
type OperationManager struct {
	client *client.APIClient
}

// NewOperationManager 创建新的操作管理器
func NewOperationManager(client *client.APIClient) *OperationManager {
	return &OperationManager{
		client: client,
	}
}

// GetDeviceList 获取设备列表
func (om *OperationManager) GetDeviceList() (interface{}, error) {
	// 发送请求
	body, err := om.client.Get("/api/v1/devices")
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析业务数据
	var deviceList models.DeviceListResponse
	if err := json.Unmarshal(apiResp.Data, &deviceList); err != nil {
		return nil, fmt.Errorf("解析设备列表失败: %w", err)
	}

	return deviceList, nil
}

// GetDeviceStatus 获取设备状态
func (om *OperationManager) GetDeviceStatus(deviceID string) (interface{}, error) {
	// 构建URL
	path := fmt.Sprintf("/api/v1/device/%s/status", deviceID)

	// 发送请求
	body, err := om.client.Get(path)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析设备状态
	var deviceInfo models.DeviceInfo
	if err := json.Unmarshal(apiResp.Data, &deviceInfo); err != nil {
		return nil, fmt.Errorf("解析设备状态失败: %w", err)
	}

	return deviceInfo, nil
}

// SendCommand 发送命令到设备
func (om *OperationManager) SendCommand(deviceID string, command byte, dataHex string) (interface{}, error) {
	// 准备请求数据
	var data []byte
	var err error
	if dataHex != "" {
		data, err = hex.DecodeString(dataHex)
		if err != nil {
			return nil, fmt.Errorf("十六进制数据格式错误: %w", err)
		}
	}

	req := models.SendCommandRequest{
		DeviceID: deviceID,
		Command:  command,
		Data:     data,
	}

	// 发送请求
	body, err := om.client.Post("/api/v1/device/command", req)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	return apiResp, nil
}

// SendDNYCommand 发送DNY协议命令
func (om *OperationManager) SendDNYCommand(deviceID string, command byte, dataHex string, waitReply bool, timeoutSec int) (interface{}, error) {
	// 准备请求数据
	req := models.DNYCommandRequest{
		DeviceID:   deviceID,
		Command:    command,
		Data:       dataHex,
		WaitReply:  waitReply,
		TimeoutSec: timeoutSec,
	}

	// 发送请求
	body, err := om.client.Post("/api/v1/command/dny", req)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析DNY命令响应
	var dnyResp models.DNYCommandResponse
	if err := json.Unmarshal(apiResp.Data, &dnyResp); err != nil {
		return nil, fmt.Errorf("解析DNY命令响应失败: %w", err)
	}

	return dnyResp, nil
}

// StartCharging 开始充电
func (om *OperationManager) StartCharging(deviceID string, portNumber, duration int, amount float64, orderNumber string, paymentType, rateMode, maxPower int) (interface{}, error) {
	// 将客户端参数转换为服务器端期望的格式
	// 充电模式映射：客户端 1=按时间 2=按电量 -> 协议 0=按时间 1=按电量
	var mode byte
	var value uint16

	if rateMode == 2 {
		// 按电量充电
		mode = byte(1)
		value = uint16(duration) // duration 在按电量模式下已经是 0.1度 单位
	} else {
		// 按时间充电（默认）
		mode = byte(0)
		value = uint16(duration / 60) // 将秒转换为分钟
		if value == 0 {
			value = 1 // 至少1分钟
		}
	}

	// 准备请求数据
	req := models.ChargingStartRequest{
		DeviceID: deviceID,
		Port:     byte(portNumber),
		Mode:     mode,
		Value:    value,
		OrderNo:  orderNumber,
		Balance:  uint32(amount * 100), // 将元转换为分
	}

	// 发送请求
	body, err := om.client.Post("/api/v1/charging/start", req)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析充电控制响应
	var chargeResp models.ChargingControlResponse
	if err := json.Unmarshal(apiResp.Data, &chargeResp); err != nil {
		return nil, fmt.Errorf("解析充电控制响应失败: %w", err)
	}

	return chargeResp, nil
}

// StopCharging 停止充电
func (om *OperationManager) StopCharging(deviceID string, portNumber int, orderNumber, reason string) (interface{}, error) {
	// 将客户端参数转换为服务器端期望的格式
	port := byte(portNumber)
	if port == 0 {
		port = 0xFF // 0xFF表示停止所有端口
	}

	// 准备请求数据
	req := models.ChargingStopRequest{
		DeviceID: deviceID,
		Port:     port,
		OrderNo:  orderNumber,
	}

	// 发送请求
	body, err := om.client.Post("/api/v1/charging/stop", req)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析充电控制响应
	var chargeResp models.ChargingControlResponse
	if err := json.Unmarshal(apiResp.Data, &chargeResp); err != nil {
		return nil, fmt.Errorf("解析充电控制响应失败: %w", err)
	}

	return chargeResp, nil
}

// QueryDeviceStatus 查询设备状态(0x81命令)
func (om *OperationManager) QueryDeviceStatus(deviceID string) (interface{}, error) {
	// 构建URL
	path := fmt.Sprintf("/api/v1/device/%s/query", deviceID)

	// 发送请求
	body, err := om.client.Get(path)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	return apiResp, nil
}

// HealthCheck 健康检查
func (om *OperationManager) HealthCheck() (interface{}, error) {
	// 发送请求
	body, err := om.client.Get("/api/v1/health")
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 解析健康检查响应
	var healthResp models.HealthResponse
	if err := json.Unmarshal(apiResp.Data, &healthResp); err != nil {
		return nil, fmt.Errorf("解析健康检查响应失败: %w", err)
	}

	return healthResp, nil
}

// GetDeviceGroupInfo 获取设备组信息
func (om *OperationManager) GetDeviceGroupInfo(deviceID string) (interface{}, error) {
	// 构建URL
	path := fmt.Sprintf("/api/v1/device/%s/group", deviceID)

	// 发送请求
	body, err := om.client.Get(path)
	if err != nil {
		return nil, err
	}

	// 解析标准响应格式
	var apiResp client.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API错误: %s", apiResp.Message)
	}

	// 返回原始数据
	return apiResp.Data, nil
}
