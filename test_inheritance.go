package main

import (
	"fmt"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

type TestHandler struct {
	protocol.SimpleHandlerBase
}

func main() {
	h := &TestHandler{}
	fmt.Printf("Methods available: %T\n", h)
	// 尝试调用方法
	fmt.Println("Test completed")
}
