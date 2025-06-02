package ports

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aceld/zinx/ziface"
)

// IoT协议解码器
type IoTDecoder struct{}

// GetLengthField 获取长度字段信息，以实现ziface.IDecoder接口
func (d *IoTDecoder) GetLengthField() *ziface.LengthField {
	return &ziface.LengthField{
		// 长度字段在包头魔术字之后
		LengthFieldOffset: IOT_HEADER_SIZE,
		// 长度字段长度为2字节
		LengthFieldLength: IOT_LENGTH_SIZE,
		// 长度调整值为0，表示长度字段后直接是消息数据
		LengthAdjustment: 0,
		// 初始跳过的字节数，包括包头魔术字和长度字段
		InitialBytesToStrip: IOT_HEADER_SIZE + IOT_LENGTH_SIZE,
		// 小端字节序
		Order: binary.LittleEndian,
	}
}

// Intercept 拦截器方法，以实现ziface.IDecoder接口
func (d *IoTDecoder) Intercept(chain ziface.IChain) ziface.IcResp {
	request := chain.Request()
	// This layer is the custom interceptor processing logic, which simply prints the input.
	// (这一层是自定义拦截器处理逻辑，这里只是简单打印输入)
	iRequest := request.(ziface.IRequest)
	fmt.Printf("拦截器方法\n %v", iRequest.GetData())
	// 将请求传递给下一个拦截器

	return chain.Proceed(chain.Request())
}

// Decode 解码从连接读取的数据
func (d *IoTDecoder) Decode(conn ziface.IConnection) (ziface.IMessage, error) {
	// 首先检查是否有特殊消息
	specialMsg, err := d.checkSpecialMessages(conn)
	if err != nil {
		return nil, err
	}
	if specialMsg != nil {
		return specialMsg, nil
	}

	// 检查是否有之前读取的数据
	var headerBuffer []byte
	peekedData, err := conn.GetProperty("PeekedData")
	if err == nil && peekedData != nil {
		// 使用之前读取的数据
		peekedBytes, ok := peekedData.([]byte)
		if ok {
			headerBuffer = peekedBytes
			// 清除已使用的数据
			conn.RemoveProperty("PeekedData")
		}
	}

	// 如果没有之前读取的数据，则读取新的包头
	if headerBuffer == nil || len(headerBuffer) < IOT_HEADER_SIZE {
		// 初始化包头缓冲区
		if headerBuffer == nil {
			headerBuffer = make([]byte, IOT_HEADER_SIZE)
		}

		// 读取剩余的包头数据
		if _, err := io.ReadFull(conn.GetTCPConnection(), headerBuffer[len(headerBuffer):IOT_HEADER_SIZE]); err != nil {
			return nil, err
		}
	}

	// 验证包头
	if !bytes.Equal(headerBuffer[:IOT_HEADER_SIZE], []byte(IOT_HEADER_MAGIC)) {
		return nil, errors.New("invalid header magic")
	}

	// 读取长度字段 (2字节)
	lenBuffer := make([]byte, IOT_LENGTH_SIZE)
	if _, err := io.ReadFull(conn.GetTCPConnection(), lenBuffer); err != nil {
		return nil, err
	}

	// 解析长度 (小端模式)
	dataLen := binary.LittleEndian.Uint16(lenBuffer)

	// 校验包长度
	if dataLen < IOT_MIN_PACKET_SIZE {
		return nil, errors.New("data length too small")
	}

	// 读取剩余数据
	dataBuffer := make([]byte, dataLen)
	if _, err := io.ReadFull(conn.GetTCPConnection(), dataBuffer); err != nil {
		return nil, err
	}

	// 提取校验和字段
	checksumPos := len(dataBuffer) - IOT_CHECKSUM_SIZE
	receivedChecksum := binary.LittleEndian.Uint16(dataBuffer[checksumPos:])

	// 计算校验和
	calculatedChecksum := CalculateChecksum(dataBuffer[:checksumPos])

	// 验证校验和
	if receivedChecksum != calculatedChecksum {
		return nil, errors.New("checksum verification failed")
	}

	// 提取物理ID、消息ID和命令
	physicalID := ExtractPhysicalID(dataBuffer)
	messageID := ExtractMessageID(dataBuffer)
	command := ExtractCommand(dataBuffer)

	// 提取数据部分
	data := dataBuffer[IOT_PHYSICAL_ID_SIZE+IOT_MESSAGE_ID_SIZE+IOT_COMMAND_SIZE : checksumPos]

	// 创建IoT消息
	msg := &IoTMessage{
		PhysicalID: physicalID,
		MessageID:  messageID,
		Command:    command,
		Data:       data,
	}

	// 更新最后心跳时间
	conn.SetProperty("LastHeartbeat", time.Now())

	return msg, nil
}

// 检查是否为特殊消息 (SIM卡号和link心跳)
func (d *IoTDecoder) checkSpecialMessages(conn ziface.IConnection) (ziface.IMessage, error) {
	// 处理特殊消息
	tempBuffer := make([]byte, IOT_SIM_CARD_LENGTH) // 使用SIM卡长度作为最大缓冲区

	// 尝试不阻塞地读取数据
	conn.GetTCPConnection().SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	n, err := conn.GetTCPConnection().Read(tempBuffer)
	// 恢复为永不超时
	conn.GetTCPConnection().SetReadDeadline(time.Time{})

	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		// 超时或其他错误，可能不是特殊消息或没有数据可读
		return nil, nil
	}

	// 调整缓冲区大小为实际读取的数据
	tempBuffer = tempBuffer[:n]

	// 检查是否为"link"心跳
	if n >= 4 && string(tempBuffer[:4]) == IOT_LINK_HEARTBEAT {
		// 是心跳消息，更新最后心跳时间
		conn.SetProperty("LastHeartbeat", time.Now())

		// 如果有额外数据，保存起来
		if n > 4 {
			conn.SetProperty("PeekedData", tempBuffer[4:])
		}

		// 创建一个特殊的心跳消息
		heartbeatMsg := CreateIoTMessage(0, 0, 0xFF, []byte(IOT_LINK_HEARTBEAT))
		return heartbeatMsg, nil
	}

	// 检查是否为SIM卡号
	if n == IOT_SIM_CARD_LENGTH && isSimCardNumber(tempBuffer) {
		// 存储SIM卡号到连接属性
		conn.SetProperty("SimCard", string(tempBuffer))

		// 创建一个特殊的SIM卡消息
		simCardMsg := CreateIoTMessage(0, 0, 0xFE, tempBuffer)
		return simCardMsg, nil
	}

	// 不是特殊消息，保存已读取的数据供后续使用
	conn.SetProperty("PeekedData", tempBuffer)
	return nil, nil
}

// 检查数据是否为SIM卡号格式
func isSimCardNumber(data []byte) bool {
	// 检查长度是否为20字节
	if len(data) != IOT_SIM_CARD_LENGTH {
		return false
	}

	// 检查是否全部为ASCII数字
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}

	return true
}

// 创建一个IoT消息
func CreateIoTMessage(physicalID uint32, messageID uint16, command uint8, data []byte) ziface.IMessage {
	return &IoTMessage{
		PhysicalID: physicalID,
		MessageID:  messageID,
		Command:    command,
		Data:       data,
	}
}
