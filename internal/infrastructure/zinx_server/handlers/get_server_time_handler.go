package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22 或 0x12)
// 0x22是设备获取服务器时间指令，0x12是主机获取服务器时间指令
type GetServerTimeHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理设备获取服务器时间请求
func (h *GetServerTimeHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备获取服务器时间请求")
}

// Handle 处理设备获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	// 🔥 强制控制台输出确保Handler被调用
	fmt.Printf("\n🎯 GetServerTimeHandler.Handle() 被调用! 时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))

	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()
	rawData := msg.GetData()

	// 🔥 强制控制台输出消息详情
	fmt.Printf("📨 消息详情: MsgID=%d(0x%X), DataLen=%d, RawDataHex=%s\n",
		msg.GetMsgID(), msg.GetMsgID(), len(rawData), hex.EncodeToString(rawData))

	// 打印请求详情 - 原始数据用于调试
	logger.WithFields(logrus.Fields{
		"msgID":      msg.GetMsgID(),
		"dataLen":    len(rawData),
		"rawDataHex": hex.EncodeToString(rawData),
	}).Error("收到获取服务器时间请求原始数据") // 使用ERROR级别确保记录

	// 尝试进行DNY消息转换
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)

	// 解析物理ID和消息ID
	var physicalId uint32
	var messageID uint16
	var commandID byte

	// 如果转换成功，使用DNY消息中的物理ID和命令ID
	if ok {
		physicalId = dnyMsg.GetPhysicalId()
		commandID = byte(dnyMsg.GetMsgID())

		// 从原始数据中提取消息ID (2字节，位于物理ID之后)
		if len(rawData) >= 11 { // 包头(3) + 长度(2) + 物理ID(4) + 消息ID(2)
			messageID = binary.LittleEndian.Uint16(rawData[9:11])
		}

		logger.WithFields(logrus.Fields{
			"command":    fmt.Sprintf("0x%02X", commandID),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"rawData":    hex.EncodeToString(rawData),
		}).Error("收到获取服务器时间请求 - 转换为DNY消息成功") // 使用ERROR级别确保记录
	} else {
		// DNY消息转换失败，尝试直接从原始数据解析
		if len(rawData) >= 11 {
			// 验证DNY包头
			if string(rawData[0:3]) != "DNY" {
				logger.WithFields(logrus.Fields{
					"header":  string(rawData[0:3]),
					"rawData": hex.EncodeToString(rawData),
				}).Error("解析DNY消息失败：无效的包头")
				return
			}

			// 从原始数据提取物理ID (4字节，小端序)
			physicalId = binary.LittleEndian.Uint32(rawData[5:9])

			// 从原始数据提取消息ID (2字节，小端序)
			messageID = binary.LittleEndian.Uint16(rawData[9:11])

			// 从原始数据提取命令ID (1字节)
			commandID = rawData[11]

			logger.WithFields(logrus.Fields{
				"command":    fmt.Sprintf("0x%02X", commandID),
				"physicalID": fmt.Sprintf("0x%08X", physicalId),
				"messageID":  fmt.Sprintf("0x%04X", messageID),
				"rawData":    hex.EncodeToString(rawData),
			}).Error("收到获取服务器时间请求 - 直接从原始数据解析") // 使用ERROR级别确保记录
		} else {
			logger.WithFields(logrus.Fields{
				"error":   "数据长度不足",
				"dataLen": len(rawData),
				"rawData": hex.EncodeToString(rawData),
			}).Error("解析DNY消息失败：数据长度不足")
			return
		}
	}

	// 构建响应消息
	// 1. 获取当前时间戳
	timestamp := uint32(time.Now().Unix())

	// 2. 构建响应数据
	// 数据长度 = 物理ID(4) + 消息ID(2) + 命令(1) + 时间戳(4) + 校验(2)
	dataLen := uint16(4 + 2 + 1 + 4 + 2)

	// 创建响应数据包
	respData := make([]byte, 0, 3+2+int(dataLen)) // 包头(3) + 长度(2) + 数据

	// 添加包头 "DNY"
	respData = append(respData, 'D', 'N', 'Y')

	// 添加长度字段 (小端序)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, dataLen)
	respData = append(respData, lenBytes...)

	// 添加物理ID (使用与请求相同的物理ID)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, physicalId)
	respData = append(respData, idBytes...)

	// 添加消息ID (使用与请求相同的消息ID)
	msgIdBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(msgIdBytes, messageID)
	respData = append(respData, msgIdBytes...)

	// 添加命令字节 (使用与请求相同的命令)
	respData = append(respData, commandID)

	// 添加时间戳 (小端序)
	timestampBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(timestampBytes, timestamp)
	respData = append(respData, timestampBytes...)

	// 计算校验和
	checksum := protocol.CalculatePacketChecksum(respData)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	respData = append(respData, checksumBytes...)

	// 打印响应详情 - 在发送前
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", commandID),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"timestamp":  timestamp,
		"respData":   hex.EncodeToString(respData),
	}).Error("准备发送服务器时间响应") // 使用ERROR级别确保记录

	// 发送响应
	err := conn.SendMsg(0, respData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("发送服务器时间响应失败")
		return
	}

	// 打印响应详情
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", commandID),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"timestamp":  timestamp,
		"dateTime":   time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
		"rawData":    hex.EncodeToString(respData),
	}).Error("已发送服务器时间响应") // 使用ERROR级别确保记录
}

// PostHandle 后处理设备获取服务器时间请求
func (h *GetServerTimeHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("设备获取服务器时间请求处理完成")
}
