package constants

// DNY协议消息ID常量
const (
	// 标准DNY协议消息ID范围: 0x00-0xFE
	// 特殊消息ID范围: 0xFF00-0xFFFF

	// 特殊消息ID
	MsgIDErrorFrame    = 0xFF00 // 错误帧消息ID
	MsgIDICCID         = 0xFF01 // ICCID消息ID
	MsgIDLinkHeartbeat = 0xFF02 // Link心跳消息ID
	MsgIDUnknown       = 0xFF03 // 未知类型消息ID
)

// 协议相关常量
const (
	IotSimCardLength = 20     // ICCID长度
	IotLinkHeartbeat = "link" // Link心跳字符串

	// 连接缓冲区管理常量
	ConnectionBufferKey = "dny_connection_buffer" // 连接缓冲区属性键

	// 消息解析常量
	LinkMessageLength  = 4      // "link"心跳消息长度
	LinkMessagePayload = "link" // Link心跳消息内容（文档兼容性）
	ICCIDMinLength     = 19     // ICCID最小长度
	ICCIDMaxLength     = 25     // ICCID最大长度
	ICCIDMessageLength = 20     // ICCID标准长度（文档兼容性）
	ICCIDValidPrefix   = "89"   // ICCID有效前缀（ITU-T E.118标准，电信行业标识符）
	DNYMinHeaderLength = 5      // DNY协议最小头部长度("DNY" + 长度字段)
	DNYChecksumLength  = 2      // DNY校验和长度
)

// 向后兼容代码已清理
// 请使用统一的命令注册表API：pkg/constants/command_definitions.go
