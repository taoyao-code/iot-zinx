package protocol

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// RawDataHook 是原始数据处理钩子
// 用于处理连接中的原始数据，例如ICCID识别、AT命令响应等
type RawDataHook struct {
	// 原始数据处理函数
	handleRawDataFunc func(conn ziface.IConnection, data []byte) bool
}

// NewRawDataHook 创建原始数据处理钩子
func NewRawDataHook(handleRawDataFunc func(conn ziface.IConnection, data []byte) bool) *RawDataHook {
	return &RawDataHook{
		handleRawDataFunc: handleRawDataFunc,
	}
}

// Handle 处理原始数据
// 返回true表示数据已处理，false表示需要继续处理
func (r *RawDataHook) Handle(conn ziface.IConnection, data []byte) bool {
	if r.handleRawDataFunc != nil {
		return r.handleRawDataFunc(conn, data)
	}
	return false
}

// DefaultRawDataHandler 默认的原始数据处理器
// 主要处理ICCID识别、AT命令响应等
func DefaultRawDataHandler(conn ziface.IConnection, data []byte) bool {
	// 尝试将数据转为字符串
	strData := string(data)

	// 检查是否为ICCID响应
	if isICCIDResponse(strData) {
		return handleICCIDResponse(conn, strData)
	}

	// 检查是否为AT命令响应
	if isATCommandResponse(strData) {
		return handleATCommandResponse(conn, strData)
	}

	// 检查是否为纯十六进制数据
	if IsHexString(data) {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"dataHex":    string(data),
		}).Info("接收到十六进制字符串数据")

		// 解码十六进制数据
		decoded, err := hex.DecodeString(string(data))
		if err == nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"decodedLen": len(decoded),
				"dataHex":    hex.EncodeToString(decoded),
			}).Debug("已解码十六进制字符串")
			return false // 继续处理解码后的数据
		}
	}

	// 未识别的数据，返回false继续处理
	return false
}

// isICCIDResponse 检查是否为ICCID响应
func isICCIDResponse(data string) bool {
	return strings.Contains(data, "ICCID:") || strings.Contains(data, "CCID:")
}

// handleICCIDResponse 处理ICCID响应
func handleICCIDResponse(conn ziface.IConnection, data string) bool {
	var iccid string

	// 提取ICCID
	if strings.Contains(data, "ICCID:") {
		parts := strings.Split(data, "ICCID:")
		if len(parts) > 1 {
			iccid = strings.TrimSpace(parts[1])
		}
	} else if strings.Contains(data, "CCID:") {
		parts := strings.Split(data, "CCID:")
		if len(parts) > 1 {
			iccid = strings.TrimSpace(parts[1])
		}
	}

	// 清理可能的回车换行
	iccid = strings.ReplaceAll(iccid, "\r", "")
	iccid = strings.ReplaceAll(iccid, "\n", "")
	iccid = strings.TrimSpace(iccid)

	if iccid != "" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"ICCID":      iccid,
		}).Info("已识别设备ICCID")

		// 使用DeviceSession统一管理连接属性
		deviceSession := session.GetDeviceSession(conn)
		if deviceSession != nil {
			deviceSession.ICCID = iccid
			deviceSession.SyncToConnection(conn)
		}

		// 响应设备 - 🔧 使用统一发送器
		response := "ICCID识别成功\r\n"
		if err := network.SendRaw(conn, []byte(response)); err != nil {
			logger.WithFields(logrus.Fields{
				"error":      err.Error(),
				"connID":     conn.GetConnID(),
				"remoteAddr": conn.RemoteAddr().String(),
			}).Error("发送ICCID响应失败")
		}

		// 返回true表示数据已处理
		return true
	}

	return false
}

// isATCommandResponse 检查是否为AT命令响应
func isATCommandResponse(data string) bool {
	return strings.HasPrefix(data, "AT") || strings.Contains(data, "OK") || strings.Contains(data, "ERROR")
}

// handleATCommandResponse 处理AT命令响应
func handleATCommandResponse(conn ziface.IConnection, data string) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"command":    strings.TrimSpace(data),
	}).Info("接收到AT命令或响应")

	// 简单的AT命令响应
	if strings.HasPrefix(strings.TrimSpace(data), "AT") {
		// 发送OK响应 - 🔧 使用统一发送器
		response := "OK\r\n"
		if err := network.SendRaw(conn, []byte(response)); err != nil {
			logger.WithFields(logrus.Fields{
				"error":      err.Error(),
				"connID":     conn.GetConnID(),
				"remoteAddr": conn.RemoteAddr().String(),
			}).Error("发送AT命令响应失败")
		}
		return true
	}

	// 已处理AT命令
	return true
}

// PrintRawData 打印原始数据，用于调试
func PrintRawData(data []byte) {
	fmt.Printf("原始数据(长度=%d): ", len(data))
	if len(data) > 0 {
		fmt.Printf("%s\n", hex.EncodeToString(data))
	} else {
		fmt.Println("空")
	}
}
