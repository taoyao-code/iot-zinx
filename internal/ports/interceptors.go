package ports

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// Custom Interceptor 1

type MyInterceptor struct{}

func (m *MyInterceptor) Intercept(chain ziface.IChain) ziface.IcResp {
	request := chain.Request()
	// This layer is the custom interceptor processing logic, which simply prints the input.
	// (这一层是自定义拦截器处理逻辑，这里只是简单打印输入)
	iRequest := request.(ziface.IRequest)
	logger.Info("MyInterceptor, Recv：%s", iRequest.GetData())
	return chain.Proceed(chain.Request())
}
