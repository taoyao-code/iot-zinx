package main

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg"
)

func main() {
	// 测试拦截器工厂是否正确配置
	fmt.Println("🧪 测试DNY协议拦截器工厂...")

	// 初始化包依赖
	pkg.InitPackages()

	// 创建拦截器工厂
	factory := pkg.Protocol.NewDNYProtocolInterceptorFactory()
	if factory == nil {
		fmt.Println("❌ 拦截器工厂创建失败")
		return
	}

	// 创建拦截器
	interceptor := factory.NewInterceptor()
	if interceptor == nil {
		fmt.Println("❌ 拦截器创建失败")
		return
	}

	fmt.Printf("✅ 拦截器创建成功，类型: %T\n", interceptor)

	// 测试数据包工厂
	dataPackFactory := pkg.Protocol.NewDNYDataPackFactory()
	if dataPackFactory == nil {
		fmt.Println("❌ 数据包工厂创建失败")
		return
	}

	dataPack := dataPackFactory.NewDataPack(true)
	if dataPack == nil {
		fmt.Println("❌ 数据包处理器创建失败")
		return
	}

	fmt.Printf("✅ 数据包处理器创建成功，类型: %T\n", dataPack)

	fmt.Println("🎉 所有组件初始化成功！拦截器架构修复完成。")
}
