package ports

// 协议常量定义
const (
	// 协议包头
	IOT_HEADER_MAGIC = "DNY"

	// 包头长度
	IOT_HEADER_SIZE = 3

	// 长度字段大小
	IOT_LENGTH_SIZE = 2

	// 物理ID大小
	IOT_PHYSICAL_ID_SIZE = 4

	// 消息ID大小
	IOT_MESSAGE_ID_SIZE = 2

	// 命令字段大小
	IOT_COMMAND_SIZE = 1

	// 校验和大小
	IOT_CHECKSUM_SIZE = 2

	// 最小包大小 = 物理ID + 消息ID + 命令 + 校验和
	IOT_MIN_PACKET_SIZE = IOT_PHYSICAL_ID_SIZE + IOT_MESSAGE_ID_SIZE + IOT_COMMAND_SIZE + IOT_CHECKSUM_SIZE

	// 特殊消息: SIM卡长度
	IOT_SIM_CARD_LENGTH = 20

	// 特殊消息: link心跳
	IOT_LINK_HEARTBEAT = "link"
)
