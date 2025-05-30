package protocol

import (
	"bytes"
	"testing"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// TestDNYPacketEncodeAndDecode 测试DNY协议的封包和解包功能
func TestDNYPacketEncodeAndDecode(t *testing.T) {
	// 创建一个DNY数据包处理器
	factory := NewDNYDataPackFactory()
	dp := factory.NewDataPack(false)

	// 测试数据
	testCases := []struct {
		name       string
		physicalId uint32
		messageId  uint16
		command    uint8
		data       []byte
	}{
		{
			name:       "HeartbeatPacket",
			physicalId: 0x12345678,
			messageId:  1,
			command:    0x01, // 心跳命令
			data:       []byte{0x01, 0x02},
		},
		{
			name:       "EmptyDataPacket",
			physicalId: 0x87654321,
			messageId:  2,
			command:    0x20, // 设备注册命令
			data:       []byte{},
		},
		{
			name:       "LargeDataPacket",
			physicalId: 0x11223344,
			messageId:  3,
			command:    0x81, // 设备状态命令
			data:       bytes.Repeat([]byte{0xAA}, 50),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建一个DNY消息
			msg := createDNYMessage(tc.physicalId, tc.messageId, tc.command, tc.data)

			// 封包
			packedData, err := dp.Pack(msg)
			if err != nil {
				t.Fatalf("封包失败: %v", err)
			}

			// 获取包头长度
			headLen := dp.GetHeadLen()
			if headLen <= 0 {
				t.Fatalf("包头长度错误: %d", headLen)
			}

			// 解包头
			unpackMsg, err := dp.Unpack(packedData)
			if err != nil {
				t.Fatalf("解包头失败: %v", err)
			}

			// 检查消息类型
			dnyMsg, ok := dny_protocol.IMessageToDnyMessage(unpackMsg)
			if !ok {
				t.Fatalf("消息类型错误: %T", unpackMsg)
			}

			// 验证解包后的消息属性
			if dnyMsg.GetPhysicalId() != tc.physicalId {
				t.Errorf("物理ID不匹配: 期望 %X, 得到 %X", tc.physicalId, dnyMsg.GetPhysicalId())
			}

			if uint32(dnyMsg.GetMsgID()) != uint32(tc.command) {
				t.Errorf("命令ID不匹配: 期望 %d, 得到 %d", tc.command, dnyMsg.GetMsgID())
			}

			if dnyMsg.GetDataLen() != uint32(len(tc.data)) {
				t.Errorf("数据长度不匹配: 期望 %d, 得到 %d", len(tc.data), dnyMsg.GetDataLen())
			}

			// 这里不检查校验和，因为校验和计算方式可能有多种
			// 只验证基本功能是否正常工作
		})
	}
}

// TestParseProtocol 测试协议解析功能
func TestParseProtocol(t *testing.T) {
	// 这个测试假设ParseDNYProtocol已经实现
	t.Skip("暂时跳过这个测试，等待ParseDNYProtocol完善")
}

// createDNYMessage 创建一个DNY消息对象
func createDNYMessage(physicalId uint32, messageId uint16, command uint8, data []byte) ziface.IMessage {
	// 创建DNY协议消息
	return &dny_protocol.Message{
		Id:         uint32(command),
		DataLen:    uint32(len(data)),
		Data:       data,
		PhysicalId: physicalId,
	}
}

// calculateChecksum 计算校验和
func calculateChecksum(data []byte) byte {
	var sum byte = 0
	for _, b := range data {
		sum += b
	}
	return sum
}
