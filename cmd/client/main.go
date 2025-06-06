package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/sirupsen/logrus"
)

// 设备启动参数
type ClientParams struct {
	simCount     int    // SIM卡数量
	devicePerSim int    // 每个SIM卡下的设备数量
	serverAddr   string // 服务器地址
	startID      uint32 // 起始物理ID
	runTests     bool   // 是否运行测试序列
	verbose      bool   // 是否输出详细日志
	mode         string // 启动模式："sim"=SIM卡模式，"device"=设备模式，"real"=真实设备模拟
	simMode      string // SIM卡模式："shared"=共享SIM卡，"individual"=独立SIM卡
	directConn   bool   // 是否启用直连模式（分机直接连接服务器）
}

func main() {
	fmt.Println("🚀 DNY协议多设备测试客户端启动")
	fmt.Println("=====================================")

	// 解析命令行参数
	params := parseFlags()

	// 初始化依赖包
	pkg.InitPackages()

	// 创建SIM卡和设备
	var simCards []*SimCard
	var clients []*TestClient

	// 设置信号处理，支持优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 根据不同模式启动设备
	if params.mode == "real" {
		// 真实设备模拟模式
		fmt.Printf("🎯 使用真实设备模拟模式：基于线上日志数据\n")
		fmt.Printf("🔌 直连模式: %v\n", params.directConn)

		// 创建多个真实设备配置
		deviceConfigs := CreateMultipleDevicesConfig()

		for i, config := range deviceConfigs {
			fmt.Printf("🚀 启动真实设备模拟 #%d: 物理ID=0x%08X, ICCID=%s\n",
				i+1, config.PhysicalID, config.ICCID)

			// 设置服务器地址
			config.ServerAddr = params.serverAddr

			// 创建客户端
			client := NewTestClient(config)

			// 启动客户端
			go func(c *TestClient, idx int) {
				if err := c.Start(); err != nil {
					fmt.Printf("❌ 真实设备模拟 #%d 启动失败: %s\n", idx+1, err)
					return
				}

				// 运行测试序列（如果需要）
				if params.runTests {
					time.Sleep(10 * time.Second) // 等待设备注册完成
					fmt.Printf("🧪 开始设备 #%d 测试序列\n", idx+1)
					c.RunTestSequence()
				}
			}(client, i)

			clients = append(clients, client)

			// 间隔启动下一个设备
			time.Sleep(3 * time.Second)
		}

		fmt.Printf("📊 总计启动: %d个真实设备模拟\n", len(deviceConfigs))

	} else if params.mode == "sim" {
		// SIM卡模式
		fmt.Printf("📱 使用SIM卡模式：%d张SIM卡，每卡%d个设备\n", params.simCount, params.devicePerSim)
		fmt.Printf("🔌 直连模式: %v\n", params.directConn)

		if params.simMode == "shared" {
			// 共享SIM卡模式（多个设备共用一个ICCID）
			fmt.Println("🔌 使用共享SIM卡模式")

			// 创建SIM卡
			for i := 0; i < params.simCount; i++ {
				// 为每张SIM卡生成ICCID
				iccid := fmt.Sprintf("8986%08d%08d", rand.Intn(100000000), i+1)

				// 创建SIM卡管理器并设置直连模式
				simCard := NewSimCard(iccid, params.serverAddr)
				simCard.SetDirectConnMode(params.directConn)

				// 为SIM卡添加多个设备
				for j := 0; j < params.devicePerSim; j++ {
					deviceID := params.startID + uint32(i*params.devicePerSim+j)
					simCard.AddDevice(deviceID)
				}

				// 启动SIM卡下的所有设备
				if err := simCard.Start(params.verbose); err != nil {
					fmt.Printf("⚠️ SIM卡 %s 启动异常: %s\n", iccid, err)
				}

				// 必要时运行测试序列
				if params.runTests {
					go func(s *SimCard) {
						time.Sleep(8 * time.Second) // 等待所有设备注册完成
						s.RunTestSequence()
					}(simCard)
				}

				// 保存SIM卡引用
				simCards = append(simCards, simCard)

				// 间隔创建下一个SIM卡
				time.Sleep(2 * time.Second)
			}

			fmt.Printf("📊 总计启动: %d张SIM卡，%d个设备\n",
				len(simCards), len(simCards)*params.devicePerSim)
		} else {
			// 独立SIM卡模式（每个设备使用独立ICCID）
			fmt.Println("🔌 使用独立SIM卡模式")

			totalDevices := 0
			for i := 0; i < params.simCount; i++ {
				// 创建单设备SIM卡
				for j := 0; j < params.devicePerSim; j++ {
					// 为每个设备生成唯一的ID和ICCID
					deviceID := params.startID + uint32(i*params.devicePerSim+j)
					iccid := fmt.Sprintf("8986%08d%08d", rand.Intn(100000000), deviceID)

					// 创建配置
					config := NewDeviceConfig().
						WithPhysicalID(deviceID).
						WithICCID(iccid).
						WithServerAddr(params.serverAddr)

					// 创建客户端
					client := NewTestClient(config)

					// 设置日志级别
					if params.verbose {
						client.logger.GetLogger().SetLevel(logrus.DebugLevel)
					}

					// 打印设备信息
					client.LogInfo()

					// 启动客户端
					if err := client.Start(); err != nil {
						fmt.Printf("❌ 设备 %08X 启动失败: %s\n", deviceID, err)
						continue
					}

					fmt.Printf("✅ 设备 %08X (ICCID: %s) 启动成功\n", deviceID, iccid)

					// 必要时运行测试序列
					if params.runTests {
						go func(c *TestClient) {
							time.Sleep(5 * time.Second) // 等待设备注册完成
							if err := c.RunTestSequence(); err != nil {
								fmt.Printf("❌ 设备 %s 测试序列执行失败: %s\n", c.GetPhysicalIDHex(), err)
							}
						}(client)
					}

					// 保存客户端引用
					clients = append(clients, client)
					totalDevices++

					// 间隔启动下一个设备
					time.Sleep(500 * time.Millisecond)
				}
			}

			fmt.Printf("📊 总计启动: %d个设备（每个设备有独立SIM卡）\n", totalDevices)
		}
	} else {
		// 设备模式（兼容原来的模式，每个设备独立）
		fmt.Printf("📱 使用设备模式：%d个独立设备\n", params.simCount*params.devicePerSim)

		totalDevices := params.simCount * params.devicePerSim
		for i := 0; i < totalDevices; i++ {
			// 为每个设备生成唯一的ID
			deviceID := params.startID + uint32(i)

			// 为每个设备生成唯一的ICCID
			iccid := fmt.Sprintf("8986%08d%08d", rand.Intn(100000000), deviceID)

			// 创建设备配置
			config := NewDeviceConfig().
				WithPhysicalID(deviceID).
				WithICCID(iccid).
				WithServerAddr(params.serverAddr)

			// 创建设备客户端
			client := NewTestClient(config)

			// 设置日志级别
			if params.verbose {
				client.logger.GetLogger().SetLevel(logrus.DebugLevel)
			} else {
				client.logger.GetLogger().SetLevel(logrus.InfoLevel)
			}

			// 保存客户端引用
			clients = append(clients, client)

			// 打印设备信息
			client.LogInfo()

			// 启动客户端
			if err := client.Start(); err != nil {
				fmt.Printf("❌ 设备 %08X 启动失败: %s\n", deviceID, err)
				continue
			}

			fmt.Printf("✅ 设备 %08X 启动成功\n", deviceID)

			// 必要时运行测试序列
			if params.runTests {
				go func(c *TestClient) {
					time.Sleep(5 * time.Second) // 等待设备注册完成
					if err := c.RunTestSequence(); err != nil {
						fmt.Printf("❌ 设备 %s 测试序列执行失败: %s\n", c.GetPhysicalIDHex(), err)
					}
				}(client)
			}

			// 间隔启动下一个设备，避免同时启动造成服务器压力
			time.Sleep(500 * time.Millisecond)
		}

		fmt.Printf("📊 总计启动: %d个独立设备\n", len(clients))
	}

	fmt.Println("💡 按 Ctrl+C 退出...")
	fmt.Println("💡 支持的退出信号: SIGINT (Ctrl+C), SIGTERM")

	// 等待退出信号
	sig := <-sigChan
	fmt.Printf("🔔 收到退出信号 %s，开始优雅关闭...\n", sig.String())

	// 停止所有SIM卡
	for _, simCard := range simCards {
		simCard.Stop()
	}

	// 停止所有独立客户端
	for _, client := range clients {
		client.Stop()
	}

	fmt.Println("🏁 程序退出")
}

// generateUniqueStartID 生成唯一的起始设备ID
func generateUniqueStartID() uint32 {
	// 使用当前时间戳生成唯一的基础ID
	timestamp := uint32(time.Now().Unix())
	// 取时间戳的低24位，并与设备识别码04组合
	deviceNumber := timestamp & 0x00FFFFFF
	return 0x04000000 | deviceNumber
}

// 解析命令行参数
func parseFlags() *ClientParams {
	params := &ClientParams{}

	flag.IntVar(&params.simCount, "sim-count", 1, "SIM卡数量")
	flag.IntVar(&params.devicePerSim, "dev-per-sim", 3, "每个SIM卡下的设备数量")
	flag.StringVar(&params.serverAddr, "server", "localhost:7054", "服务器地址")
	defaultStartID := generateUniqueStartID()
	startIDVar := uint(defaultStartID)
	flag.UintVar(&startIDVar, "start-id", uint(defaultStartID), "起始物理ID (十六进制)")
	params.startID = uint32(startIDVar)
	flag.BoolVar(&params.runTests, "test", false, "是否运行测试序列")
	flag.BoolVar(&params.verbose, "verbose", false, "是否输出详细日志")
	flag.StringVar(&params.mode, "mode", "real", "启动模式: sim=SIM卡模式, device=设备模式, real=真实设备模拟模式")
	flag.StringVar(&params.simMode, "sim-mode", "shared", "SIM卡模式: shared=共享SIM卡, individual=独立SIM卡")
	flag.BoolVar(&params.directConn, "direct", true, "是否启用直连模式（所有设备直接连接服务器）")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [选项]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  【真实设备模拟模式】基于线上日志数据:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode real -server localhost:7054 -test\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  【共享SIM卡模式 - 直连】每个设备都直接连接服务器:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode sim -sim-mode shared -sim-count 2 -dev-per-sim 3 -direct=true\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  【共享SIM卡模式 - 传统】只有主设备连接服务器:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode sim -sim-mode shared -sim-count 2 -dev-per-sim 3 -direct=false\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  【独立SIM卡模式】每个设备有独立SIM卡:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode sim -sim-mode individual -sim-count 1 -dev-per-sim 5\n\n", os.Args[0])
	}

	flag.Parse()

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	return params
}
