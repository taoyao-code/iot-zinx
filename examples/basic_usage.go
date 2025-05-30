package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/pkg"
)

// 示例：使用pkg包中的功能创建一个简单的TCP服务器
// 演示如何使用pkg包中的功能
func main() {
	// 初始化pkg包依赖关系
	pkg.InitPackages()

	// 创建一个zinx服务器
	s := znet.NewServer()

	// 注册连接钩子
	hooks := pkg.Network.NewConnectionHooks(
		60*time.Second,  // 读超时
		60*time.Second,  // 写超时
		120*time.Second, // KeepAlive周期
	)

	// 设置连接钩子
	s.SetOnConnStart(hooks.OnConnectionStart)
	s.SetOnConnStop(hooks.OnConnectionStop)

	// 设置监控器钩子
	tcpMonitor := pkg.Monitor.GetGlobalMonitor()
	s.SetOnConnStart(tcpMonitor.OnConnectionEstablished)
	s.SetOnConnStop(tcpMonitor.OnConnectionClosed)

	// 注册设备状态处理路由
	s.AddRouter(0x81, &DeviceStatusRouter{})

	// 获取命令管理器
	cmdMgr := pkg.Network.GetCommandManager()
	fmt.Printf("命令管理器已初始化: %v\n", cmdMgr != nil)

	// 显示服务器信息
	fmt.Println("服务器已初始化，使用pkg包中的功能")
	fmt.Println("- 已设置连接钩子")
	fmt.Println("- 已设置TCP监控器")
	fmt.Println("- 已启动命令管理器")
	fmt.Println("- 已注册设备状态查询路由(0x81)")
	fmt.Println("服务器监听在 0.0.0.0:7054")

	// 在新的goroutine中启动服务器
	go func() {
		s.Serve()
	}()

	// 等待信号退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 关闭服务器
	s.Stop()
}

// DeviceStatusRouter 设备状态查询路由处理器
type DeviceStatusRouter struct {
	znet.BaseRouter
}

// Handle 处理设备状态查询
func (r *DeviceStatusRouter) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	fmt.Printf("收到设备状态查询 - ConnID: %d, MsgID: %d\n", conn.GetConnID(), msg.GetMsgID())

	// 假设从消息中获取物理ID
	physicalID := uint32(conn.GetConnID())

	// 构建响应数据
	responseData := []byte{0x00} // 0x00 表示成功

	// 发送DNY响应
	err := pkg.Protocol.SendDNYResponse(conn, physicalID, 0, 0x81, responseData)
	if err != nil {
		fmt.Printf("发送响应失败: %v\n", err)
	}
}
