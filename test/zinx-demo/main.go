package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// 自定义消息结构体
type Message struct {
	Id      uint32 // 消息ID
	DataLen uint32 // 数据长度
	Data    []byte // 数据内容
	RawData []byte // 原始数据
}

// 实现IMessage接口的方法
func (m *Message) GetMsgID() uint32 {
	return m.Id
}

func (m *Message) GetDataLen() uint32 {
	return m.DataLen
}

func (m *Message) GetData() []byte {
	return m.Data
}

func (m *Message) GetRawData() []byte {
	return m.RawData
}

func (m *Message) SetMsgID(id uint32) {
	m.Id = id
}

func (m *Message) SetDataLen(dataLen uint32) {
	m.DataLen = dataLen
}

func (m *Message) SetData(data []byte) {
	m.Data = data
	m.DataLen = uint32(len(data))
}

// 自定义消息创建函数
func NewMessage(id uint32, data []byte) *Message {
	return &Message{
		Id:      id,
		DataLen: uint32(len(data)),
		Data:    data,
	}
}

// 自定义数据包处理器
type MyDataPack struct{}

// GetHeadLen 获取消息头长度
func (dp *MyDataPack) GetHeadLen() uint32 {
	fmt.Println("MyDataPack.GetHeadLen被调用，返回头长度: 5")
	return 5 // 例如，头部长度为5字节
}

// Pack 封包方法
func (dp *MyDataPack) Pack(msg ziface.IMessage) ([]byte, error) {
	fmt.Printf("MyDataPack.Pack被调用，消息ID: %d\n", msg.GetMsgID())

	// 创建一个存放bytes字节的缓冲
	dataBuff := bytes.NewBuffer([]byte{})

	// 写入包头标志(3字节) - 例如"DNY"
	if _, err := dataBuff.WriteString("DNY"); err != nil {
		return nil, err
	}

	// 写入数据长度(2字节)
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(msg.GetDataLen())); err != nil {
		return nil, err
	}

	// 写入消息ID
	if err := binary.Write(dataBuff, binary.LittleEndian, msg.GetMsgID()); err != nil {
		return nil, err
	}

	// 写入数据
	if err := binary.Write(dataBuff, binary.LittleEndian, msg.GetData()); err != nil {
		return nil, err
	}

	return dataBuff.Bytes(), nil
}

// Unpack 拆包方法
func (dp *MyDataPack) Unpack(binaryData []byte) (ziface.IMessage, error) {
	fmt.Printf("MyDataPack.Unpack被调用，数据长度: %d\n", len(binaryData))

	// 检查包头标志
	if len(binaryData) < 5 || string(binaryData[0:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头标志")
	}

	// 解析数据长度
	dataLen := binary.LittleEndian.Uint16(binaryData[3:5])

	// 检查数据包长度
	if uint16(len(binaryData)) < 5+dataLen {
		return nil, fmt.Errorf("数据长度不足")
	}

	// 解析消息ID
	msgID := binary.LittleEndian.Uint32(binaryData[5:9])

	// 创建消息对象
	msg := NewMessage(msgID, binaryData[9:9+dataLen])
	msg.RawData = binaryData[:9+dataLen]

	return msg, nil
}

// 自定义路由
type PingRouter struct {
	znet.BaseRouter
}

// Handle 处理消息
func (p *PingRouter) Handle(request ziface.IRequest) {
	fmt.Printf("PingRouter.Handle被调用，消息ID: %d\n", request.GetMsgID())

	// 从请求中获取数据
	fmt.Printf("收到消息: %s\n", string(request.GetData()))

	// 回复消息
	err := request.GetConnection().SendMsg(1, []byte("pong"))
	if err != nil {
		fmt.Println("回复消息失败:", err)
	}
}

// 连接建立时的钩子函数
func OnConnectionStart(conn ziface.IConnection) {
	fmt.Printf("OnConnectionStart: 连接ID=%d已建立\n", conn.GetConnID())

	// 设置连接属性
	conn.SetProperty("ConnTime", time.Now().Format("2006-01-02 15:04:05"))

	// 模拟接收初始数据
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("模拟接收初始数据")
		if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
			buffer := make([]byte, 1024)
			tcpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := tcpConn.Read(buffer)
			if err != nil && err != io.EOF {
				fmt.Println("读取初始数据失败:", err)
				return
			}
			if n > 0 {
				fmt.Printf("读取到初始数据: %v\n", buffer[:n])
			}
		}
	}()
}

// 连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	fmt.Printf("OnConnectionStop: 连接ID=%d已断开\n", conn.GetConnID())

	// 获取连接属性
	if property, err := conn.GetProperty("ConnTime"); err == nil {
		fmt.Printf("连接建立时间: %s\n", property.(string))
	}
}

func main() {
	// 设置Zinx全局配置
	zconf.GlobalObject.Name = "ZinxTestServer"
	zconf.GlobalObject.Host = "0.0.0.0"
	zconf.GlobalObject.TCPPort = 7777
	zconf.GlobalObject.Version = "1.0"
	zconf.GlobalObject.MaxConn = 10
	zconf.GlobalObject.MaxPacketSize = 4096
	zconf.GlobalObject.WorkerPoolSize = 10
	zconf.GlobalObject.MaxWorkerTaskLen = 1024

	// 创建服务器实例
	server := znet.NewServer()

	// 设置自定义数据包处理器
	fmt.Println("设置自定义数据包处理器")
	server.SetPacket(&MyDataPack{})

	// 获取数据包处理器验证是否设置成功
	packet := server.GetPacket()
	if packet != nil {
		fmt.Printf("成功获取设置的数据包处理器: %T\n", packet)
	} else {
		fmt.Println("警告: 数据包处理器设置失败或无法获取!")
	}

	// 输出服务器配置信息
	fmt.Printf("服务器配置: MaxConn=%d, WorkerPoolSize=%d\n",
		zconf.GlobalObject.MaxConn, zconf.GlobalObject.WorkerPoolSize)

	// 设置连接的钩子函数
	server.SetOnConnStart(OnConnectionStart)
	server.SetOnConnStop(OnConnectionStop)

	// 注册路由
	fmt.Println("注册Ping路由")
	server.AddRouter(1, &PingRouter{})

	// 启动服务器
	fmt.Println("启动Zinx服务器...")
	go server.Serve()

	// 等待退出信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// 停止服务器
	fmt.Println("停止Zinx服务器...")
	server.Stop()
}
