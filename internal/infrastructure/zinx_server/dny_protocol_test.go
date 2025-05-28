package zinx_server

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"testing"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/gorilla/websocket"
)

// 模拟连接结构体
type MockConnection struct {
	connID     uint64
	properties map[string]interface{}
	sentData   [][]byte // 记录发送的数据
}

func NewMockConnection(connID uint64) *MockConnection {
	return &MockConnection{
		connID:     connID,
		properties: make(map[string]interface{}),
		sentData:   make([][]byte, 0),
	}
}

func (m *MockConnection) GetConnID() uint64 {
	return m.connID
}

func (m *MockConnection) RemoteAddr() net.Addr {
	return &mockAddr{addr: "192.168.1.100:12345"}
}

type mockAddr struct {
	addr string
}

func (ma *mockAddr) String() string {
	return ma.addr
}

func (ma *mockAddr) Network() string {
	return "tcp"
}

func (m *MockConnection) SetProperty(key string, value interface{}) {
	m.properties[key] = value
}

func (m *MockConnection) GetProperty(key string) (interface{}, error) {
	if value, exists := m.properties[key]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("property %s not found", key)
}

func (m *MockConnection) SendBuffMsg(msgID uint32, data []byte) error {
	fmt.Printf("模拟发送数据 (msgID=%d): %s\n", msgID, hex.EncodeToString(data))
	m.sentData = append(m.sentData, data)
	return nil
}

func (m *MockConnection) SendMsg(msgID uint32, data []byte) error {
	return m.SendBuffMsg(msgID, data)
}

// 实现 IConnection 接口的其他方法
func (m *MockConnection) Start()                                                     {}
func (m *MockConnection) Stop()                                                      {}
func (m *MockConnection) Context() context.Context                                   { return context.Background() }
func (m *MockConnection) GetName() string                                            { return "MockConnection" }
func (m *MockConnection) GetConnection() net.Conn                                    { return nil }
func (m *MockConnection) GetWsConn() *websocket.Conn                                 { return nil }
func (m *MockConnection) GetTCPConnection() net.Conn                                 { return nil }
func (m *MockConnection) GetConnIdStr() string                                       { return fmt.Sprintf("%d", m.connID) }
func (m *MockConnection) GetMsgHandler() ziface.IMsgHandle                           { return nil }
func (m *MockConnection) GetWorkerID() uint32                                        { return 0 }
func (m *MockConnection) LocalAddr() net.Addr                                        { return &mockAddr{addr: "127.0.0.1:8080"} }
func (m *MockConnection) LocalAddrString() string                                    { return "127.0.0.1:8080" }
func (m *MockConnection) RemoteAddrString() string                                   { return "192.168.1.100:12345" }
func (m *MockConnection) Send(data []byte) error                                     { return nil }
func (m *MockConnection) SendToQueue(data []byte) error                              { return nil }
func (m *MockConnection) RemoveProperty(key string)                                  { delete(m.properties, key) }
func (m *MockConnection) IsAlive() bool                                              { return true }
func (m *MockConnection) SetHeartBeat(checker ziface.IHeartbeatChecker)              {}
func (m *MockConnection) AddCloseCallback(handler, key interface{}, callback func()) {}
func (m *MockConnection) RemoveCloseCallback(handler, key interface{})               {}
func (m *MockConnection) InvokeCloseCallbacks()                                      {}

// 测试真实的设备数据包处理
func TestRealDevicePackets(t *testing.T) {
	// 从日志中提取的真实数据包
	testPackets := []struct {
		name        string
		hexData     string
		description string
	}{
		{
			name:        "ICCID",
			hexData:     "3839383630344439313632333930343838323937",
			description: "设备ICCID数据",
		},
		{
			name:        "HeartbeatPacket1",
			hexData:     "444e591d00f36ca2047d01018002260902000000000000000000000a00315d00d704",
			description: "设备心跳数据包1",
		},
		{
			name:        "HeartbeatPacket2",
			hexData:     "444e591d00cd28a2046a01018002200902000000000000000000001e00315a006504",
			description: "设备心跳数据包2",
		},
		{
			name:        "LinkHeartbeat",
			hexData:     "6c696e6b",
			description: "Link心跳包",
		},
		{
			name:        "DeviceRegister",
			hexData:     "444e595000f36ca204030011680202742e37681a07383938363034443931363233393034383832393755000038363434353230363937363234373256312e302e30302e3030303030302e3036313600000000008610",
			description: "设备注册数据包",
		},
		{
			name:        "GetServerTime",
			hexData:     "444e590900f36ca2047f01229b03",
			description: "获取服务器时间请求",
		},
	}

	for _, packet := range testPackets {
		t.Run(packet.name, func(t *testing.T) {
			fmt.Printf("\n=== 测试 %s ===\n", packet.description)
			fmt.Printf("原始数据: %s\n", packet.hexData)

			// 解码十六进制数据
			data, err := hex.DecodeString(packet.hexData)
			if err != nil {
				t.Fatalf("十六进制解码失败: %v", err)
			}

			// 创建模拟连接
			mockConn := NewMockConnection(1)

			// 测试HandlePacket函数
			handled := HandlePacket(mockConn, data)

			fmt.Printf("处理结果: %v\n", handled)
			if len(mockConn.sentData) > 0 {
				fmt.Printf("响应数据: %s\n", hex.EncodeToString(mockConn.sentData[0]))
			} else {
				fmt.Printf("未发送响应数据\n")
			}

			// 验证是否正确处理
			if !handled {
				t.Logf("警告: 数据包 %s 未被正确处理", packet.name)
			}
		})
	}
}

// 测试DNY协议解析
func TestDNYProtocolParsing(t *testing.T) {
	// 测试心跳包解析
	heartbeatHex := "444e591d00f36ca2047d01018002260902000000000000000000000a00315d00d704"
	data, err := hex.DecodeString(heartbeatHex)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 创建DNY数据包解析器
	packet := &DNYPacket{}
	msg, err := packet.Unpack(data)
	if err != nil {
		t.Fatalf("DNY协议解析失败: %v", err)
	}

	// 转换为DNY消息
	if dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg); ok {
		fmt.Printf("物理ID: 0x%08X\n", dnyMsg.GetPhysicalId())
		fmt.Printf("消息ID: 0x%04X\n", dnyMsg.GetMsgID())
		fmt.Printf("命令: 0x%02X\n", dnyMsg.GetData()[0]) // 假设第一个字节是命令
		fmt.Printf("数据长度: %d\n", len(dnyMsg.GetData()))
	} else {
		t.Fatalf("消息类型转换失败")
	}
}

// 测试响应发送功能
func TestResponseSending(t *testing.T) {
	// 模拟设备注册响应
	mockConn := NewMockConnection(1)
	physicalID := uint32(0xa2f36c)
	messageID := uint16(0x0403)
	command := uint8(dny_protocol.CmdDeviceRegister)
	responseData := []byte{0x00, 0x02, 0x00, 0x00, 0x00} // 成功响应

	err := SendDNYResponse(mockConn, physicalID, messageID, command, responseData)
	if err != nil {
		t.Fatalf("发送响应失败: %v", err)
	}

	if len(mockConn.sentData) == 0 {
		t.Fatalf("未发送任何数据")
	}

	// 验证发送的数据格式
	sentData := mockConn.sentData[0]
	fmt.Printf("发送的响应数据: %s\n", hex.EncodeToString(sentData))

	// 检查DNY协议头
	if len(sentData) < 3 || string(sentData[:3]) != "DNY" {
		t.Fatalf("响应数据格式错误，缺少DNY头")
	}

	fmt.Printf("响应发送测试通过\n")
}

// 测试连接属性管理
func TestConnectionProperties(t *testing.T) {
	mockConn := NewMockConnection(1)

	// 测试设备ID绑定
	deviceID := "A2F36C00"
	BindDeviceIdToConnection(deviceID, mockConn)

	// 验证属性设置
	if storedID, err := mockConn.GetProperty(PropKeyDeviceId); err != nil {
		t.Fatalf("获取设备ID属性失败: %v", err)
	} else if storedID != deviceID {
		t.Fatalf("设备ID不匹配: 期望 %s, 实际 %s", deviceID, storedID)
	}

	// 测试ICCID设置
	iccid := "89860439234820399456"
	mockConn.SetProperty(PropKeyICCID, iccid)

	if storedICCID, err := mockConn.GetProperty(PropKeyICCID); err != nil {
		t.Fatalf("获取ICCID属性失败: %v", err)
	} else if storedICCID != iccid {
		t.Fatalf("ICCID不匹配: 期望 %s, 实际 %s", iccid, storedICCID)
	}

	fmt.Printf("连接属性管理测试通过\n")
}

// 基准测试 - 测试数据包处理性能
func BenchmarkPacketProcessing(b *testing.B) {
	heartbeatHex := "444e591d00f36ca2047d01018002260902000000000000000000000a00315d00d704"
	data, _ := hex.DecodeString(heartbeatHex)
	mockConn := NewMockConnection(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HandlePacket(mockConn, data)
	}
}
