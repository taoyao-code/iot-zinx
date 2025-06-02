package protocol

import "github.com/aceld/zinx/ziface"

// IInterceptorFactory 拦截器工厂接口
type IInterceptorFactory interface {
	// NewInterceptor 创建一个新的拦截器
	NewInterceptor() ziface.IInterceptor
}

// DNYProtocolInterceptorFactory DNY协议拦截器工厂
type DNYProtocolInterceptorFactory struct{}

// NewDNYProtocolInterceptorFactory 创建DNY协议拦截器工厂
func NewDNYProtocolInterceptorFactory() IInterceptorFactory {
	return &DNYProtocolInterceptorFactory{}
}

// NewInterceptor 创建DNY协议拦截器
func (f *DNYProtocolInterceptorFactory) NewInterceptor() ziface.IInterceptor {
	return NewDNYProtocolInterceptor()
}
