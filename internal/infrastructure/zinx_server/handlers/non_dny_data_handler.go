package handlers

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// NonDNYDataHandler 处理非DNY协议数据的处理器
// 用于处理ICCID、link心跳等非DNY协议格式的数据
type NonDNYDataHandler struct{}

// NewNonDNYDataHandler 创建非DNY数据处理器
func NewNonDNYDataHandler() ziface.IRouter {
	return &NonDNYDataHandler{}
}

// PreHandle 预处理
func (h *NonDNYDataHandler) PreHandle(request ziface.IRequest) {
	// 可以在这里添加预处理逻辑，比如认证、限流等
}

// Handle 处理非DNY协议数据
func (h *NonDNYDataHandler) Handle(request ziface.IRequest) {
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 强制输出调试信息
	fmt.Printf("\n🔥🔥🔥 NonDNYDataHandler.Handle被调用! msgID: %d 🔥🔥🔥\n", msg.GetMsgID())
	fmt.Printf("数据长度: %d\n", msg.GetDataLen())
	fmt.Printf("数据(HEX): %s\n", hex.EncodeToString(msg.GetData()))

	// 转换为DNY消息以获取原始数据
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理非DNY协议数据")
		return
	}

	// 获取原始数据
	data := dnyMsg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"dataStr":    string(data),
	}).Info("处理非DNY协议数据")

	// 处理不同类型的非DNY协议数据
	processed := h.processNonDNYData(conn, data)

	if !processed {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"dataHex":    hex.EncodeToString(data),
		}).Warn("未能识别的非DNY协议数据")
	}
}

// PostHandle 后处理
func (h *NonDNYDataHandler) PostHandle(request ziface.IRequest) {
	// 可以在这里添加后处理逻辑，比如清理、统计等
}

// processNonDNYData 处理具体的非DNY协议数据
func (h *NonDNYDataHandler) processNonDNYData(conn ziface.IConnection, data []byte) bool {
	// 1. 处理ICCID (20字节数字字符串)
	if len(data) == 20 && h.isValidICCIDBytes(data) {
		return h.processICCID(conn, data)
	}

	// 2. 处理link心跳
	if len(data) == 4 && string(data) == zinx_server.LinkHeartbeat {
		return h.processLinkHeartbeat(conn, data)
	}

	// 3. 处理十六进制编码数据
	if h.isHexEncodedData(data) {
		return h.processHexEncodedData(conn, data)
	}

	// 4. 处理其他未知数据
	return h.processUnknownData(conn, data)
}

// processICCID 处理ICCID数据
func (h *NonDNYDataHandler) processICCID(conn ziface.IConnection, data []byte) bool {
	iccidStr := string(data)
	conn.SetProperty(zinx_server.PropKeyICCID, iccidStr)

	// 将ICCID作为设备ID进行绑定
	zinx_server.BindDeviceIdToConnection(iccidStr, conn)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccidStr,
	}).Info("收到并处理ICCID数据")

	fmt.Printf("✅ ICCID处理成功: %s\n", iccidStr)
	return true
}

// processLinkHeartbeat 处理link心跳
func (h *NonDNYDataHandler) processLinkHeartbeat(conn ziface.IConnection, data []byte) bool {
	// 更新心跳时间（无返回值）
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 手动获取当前时间戳用于设置link属性
	now := time.Now().Unix()
	conn.SetProperty(zinx_server.PropKeyLastLink, now)
	conn.SetProperty(zinx_server.PropKeyConnStatus, zinx_server.ConnStatusActive)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"heartbeat":  string(data),
		"timestamp":  now,
	}).Info("收到并处理link心跳")

	fmt.Printf("✅ Link心跳处理成功: %s\n", string(data))
	return true
}

// processHexEncodedData 处理十六进制编码数据
func (h *NonDNYDataHandler) processHexEncodedData(conn ziface.IConnection, data []byte) bool {
	// 解码十六进制字符串
	decoded, err := hex.DecodeString(string(data))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"error":      err.Error(),
			"dataHex":    hex.EncodeToString(data),
		}).Error("十六进制解码失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"remoteAddr":  conn.RemoteAddr().String(),
		"originalLen": len(data),
		"decodedLen":  len(decoded),
		"decodedHex":  hex.EncodeToString(decoded),
	}).Info("处理十六进制编码数据")

	// 递归处理解码后的数据
	return h.processNonDNYData(conn, decoded)
}

// processUnknownData 处理未知数据
func (h *NonDNYDataHandler) processUnknownData(conn ziface.IConnection, data []byte) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"dataStr":    string(data),
	}).Debug("收到未知的非DNY协议数据")

	fmt.Printf("❓ 未知数据: 长度=%d, HEX=%s, ASCII=%s\n",
		len(data), hex.EncodeToString(data), string(data))

	// 即使是未知数据，也返回true表示已处理，避免错误日志
	return true
}

// isValidICCIDBytes 验证字节数组是否为有效的ICCID格式
func (h *NonDNYDataHandler) isValidICCIDBytes(data []byte) bool {
	// ICCID长度必须为20字节
	if len(data) != 20 {
		return false
	}

	// 检查每个字节是否为ASCII数字字符
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}

	return true
}

// isHexEncodedData 检查数据是否为十六进制编码的字符串
func (h *NonDNYDataHandler) isHexEncodedData(data []byte) bool {
	// 特殊情况处理：很短的数据通常不是十六进制编码
	if len(data) < 6 {
		return false
	}

	// 如果数据以"DNY"开头，不认为是十六进制编码
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		return false
	}

	// 必须是偶数长度且长度大于0
	if len(data) == 0 || len(data)%2 != 0 {
		return false
	}

	// 检查是否都是ASCII十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}
