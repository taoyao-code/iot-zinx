package handlers

import (
	"encoding/binary"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// MainServerTimeRouter 主机服务器时间处理器
// 处理0x12指令：主机获取服务器时间
type MainServerTimeRouter struct {
	znet.BaseRouter
	*BaseHandler
}

// NewMainServerTimeRouter 创建主机服务器时间处理器
func NewMainServerTimeRouter() *MainServerTimeRouter {
	return &MainServerTimeRouter{
		BaseHandler: NewBaseHandler("MainServerTimeRouter"),
	}
}

// PreHandle 预处理
func (r *MainServerTimeRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理主机获取服务器时间请求
func (r *MainServerTimeRouter) Handle(request ziface.IRequest) {
	r.Log("收到主机获取服务器时间请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是主机服务器时间请求
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeMainGetServerTime); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 构建时间响应
	response := r.BuildMainServerTimeResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID), parsedMsg.MessageID)
	
	// 发送响应
	r.SendSuccessResponse(request, response)

	r.Log("主机服务器时间响应已发送: %s", deviceID)
}

// PostHandle 后处理
func (r *MainServerTimeRouter) PostHandle(request ziface.IRequest) {}

// BuildMainServerTimeResponse 构建主机服务器时间响应包
func (r *MainServerTimeRouter) BuildMainServerTimeResponse(physicalID string, messageID uint16) []byte {
	// DNY协议响应格式: DNY(3) + Length(2) + PhysicalID(4) + MessageID(2) + Command(1) + Timestamp(4) + Checksum(2)
	response := make([]byte, 18)

	// 包头 "DNY"
	copy(response[0:3], []byte("DNY"))

	// 长度字段 (PhysicalID + MessageID + Command + Timestamp + Checksum = 12)
	binary.LittleEndian.PutUint16(response[3:5], 12)

	// 物理ID (4字节)
	physicalIDValue, _ := utils.ParsePhysicalID(physicalID)
	binary.LittleEndian.PutUint32(response[5:9], physicalIDValue)

	// 消息ID (2字节)
	binary.LittleEndian.PutUint16(response[9:11], messageID)

	// 命令字 (1字节) - 0x12
	response[11] = 0x12

	// 当前时间戳 (4字节) - Unix时间戳
	currentTime := uint32(time.Now().Unix())
	binary.LittleEndian.PutUint32(response[12:16], currentTime)

	// 计算校验和
	checksum := r.CalculateChecksum(response[5:16])
	binary.LittleEndian.PutUint16(response[16:18], checksum)

	return response
}

// CalculateChecksum 计算校验和
func (r *MainServerTimeRouter) CalculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}
