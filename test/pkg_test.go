package test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/stretchr/testify/assert"
)

// 定义错误常量
var ErrPropertyNotFound = errors.New("property not found")

// 测试初始化包依赖
func TestInitPackages(t *testing.T) {
	// 初始化包依赖关系
	pkg.InitPackages()

	// 验证命令管理器已初始化
	cmdMgr := pkg.Network.GetCommandManager()
	assert.NotNil(t, cmdMgr, "命令管理器应该已初始化")

	// 验证监控器已初始化
	mon := pkg.Monitor.GetGlobalMonitor()
	assert.NotNil(t, mon, "监控器应该已初始化")
}

// 测试心跳包监控器
func TestHeartbeatMonitor(t *testing.T) {
	// 初始化包依赖关系
	pkg.InitPackages()

	// 创建一个模拟连接
	conn := newMockConnection()

	// 绑定设备ID到连接
	deviceId := "12345678"
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 验证设备ID绑定成功
	gotConn, ok := pkg.Monitor.GetGlobalMonitor().GetConnectionByDeviceId(deviceId)
	assert.True(t, ok, "应该能够找到绑定的连接")
	assert.Equal(t, conn.GetConnID(), gotConn.GetConnID(), "连接ID应该匹配")

	// 验证设备状态更新
	pkg.Monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceId, pkg.DeviceStatusOnline)
}

// 测试DNY协议解析
func TestDNYProtocolParsing(t *testing.T) {
	// 初始化包依赖关系
	pkg.InitPackages()

	// 创建一个DNY数据包
	factory := pkg.Protocol.NewDNYDataPackFactory()
	assert.NotNil(t, factory, "数据包工厂应该已初始化")

	dp := factory.NewDataPack(true)
	assert.NotNil(t, dp, "数据包应该已创建")

	// 测试协议解析
	testData := []byte{0x68, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x01, 0x01, 0x16}
	result := pkg.Protocol.ParseDNYProtocol(testData)
	assert.NotEmpty(t, result, "解析结果不应为空")
}

// 测试心跳检测
func TestConnectionHooks(t *testing.T) {
	// 初始化包依赖关系
	pkg.InitPackages()

	// 创建连接钩子
	hooks := pkg.Network.NewConnectionHooks(
		10*time.Second, // 读超时
		10*time.Second, // 写超时
		20*time.Second, // KeepAlive周期
	)
	assert.NotNil(t, hooks, "连接钩子应该已创建")

	// 直接测试monitor的钩子函数
	conn := newMockConnection()
	tcpMonitor := pkg.Monitor.GetGlobalMonitor()

	// 测试连接建立和关闭
	tcpMonitor.OnConnectionEstablished(conn)
	tcpMonitor.OnConnectionClosed(conn)
}

// 模拟连接实现
type mockConnection struct {
	znet.Connection
	connID   uint64
	isClosed bool
	props    map[string]interface{}
}

// 创建新的模拟连接
func newMockConnection() *mockConnection {
	return &mockConnection{
		connID:   1,
		isClosed: false,
		props:    make(map[string]interface{}),
	}
}

// GetConnID 获取连接ID
func (c *mockConnection) GetConnID() uint64 {
	return c.connID
}

// IsClosed 连接是否已关闭
func (c *mockConnection) IsClosed() bool {
	return c.isClosed
}

// SetProperty 设置属性
func (c *mockConnection) SetProperty(key string, value interface{}) {
	c.props[key] = value
}

// GetProperty 获取属性
func (c *mockConnection) GetProperty(key string) (interface{}, error) {
	if value, ok := c.props[key]; ok {
		return value, nil
	}
	return nil, ErrPropertyNotFound
}

// RemoveProperty 移除属性
func (c *mockConnection) RemoveProperty(key string) {
	delete(c.props, key)
}

// Send 发送数据
func (c *mockConnection) Send(data []byte) error {
	return nil
}

// RemoteAddr 获取远程地址
func (c *mockConnection) RemoteAddr() net.Addr {
	return &mockAddr{}
}

// 模拟网络地址
type mockAddr struct{}

func (a *mockAddr) Network() string {
	return "tcp"
}

func (a *mockAddr) String() string {
	return "127.0.0.1:8999"
}
