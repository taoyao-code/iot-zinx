package protocol

import (
	"github.com/aceld/zinx/ziface"
)

// IDataPackFactory 定义了数据包工厂接口
type IDataPackFactory interface {
	// NewDataPack 创建一个数据包处理器
	NewDataPack(logHexDump bool) ziface.IDataPack
}

// DNYDataPackFactory 是DNY协议数据包工厂的实现
type DNYDataPackFactory struct{}

// NewDataPack 创建一个DNY协议数据包处理器
func (factory *DNYDataPackFactory) NewDataPack(logHexDump bool) ziface.IDataPack {
	return NewDNYPacket(logHexDump)
}

// NewDNYDataPackFactory 创建一个DNY协议数据包工厂
func NewDNYDataPackFactory() IDataPackFactory {
	return &DNYDataPackFactory{}
}
