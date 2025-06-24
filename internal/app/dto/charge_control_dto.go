package dto

import (
	"encoding/binary"
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// ChargeControlRequest 充电控制请求DTO - 统一的充电控制请求数据结构
type ChargeControlRequest struct {
	DeviceID          string `json:"deviceId" binding:"required"`      // 设备ID
	RateMode          byte   `json:"rateMode"`                         // 费率模式 (0=按时间，1=按电量)
	Balance           uint32 `json:"balance"`                          // 余额/有效期
	PortNumber        byte   `json:"portNumber" binding:"required"`    // 端口号
	ChargeCommand     byte   `json:"chargeCommand" binding:"required"` // 充电命令
	ChargeDuration    uint16 `json:"chargeDuration"`                   // 充电时长/电量
	OrderNumber       string `json:"orderNumber" binding:"required"`   // 订单编号
	MaxChargeDuration uint16 `json:"maxChargeDuration"`                // 最大充电时长
	MaxPower          uint16 `json:"maxPower"`                         // 过载功率
	QRCodeLight       byte   `json:"qrCodeLight"`                      // 二维码灯
}

// ChargeControlResponse 充电控制响应DTO - 统一的充电控制响应数据结构
type ChargeControlResponse struct {
	DeviceID       string `json:"deviceId"`       // 设备ID
	ResponseStatus byte   `json:"responseStatus"` // 响应状态
	StatusDesc     string `json:"statusDesc"`     // 状态描述
	OrderNumber    string `json:"orderNumber"`    // 订单编号
	PortNumber     byte   `json:"portNumber"`     // 端口号
	WaitPorts      uint16 `json:"waitPorts"`      // 待充端口
	Timestamp      int64  `json:"timestamp"`      // 响应时间戳
}

// Validate 验证充电控制请求参数
func (req *ChargeControlRequest) Validate() error {
	if req.DeviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if req.PortNumber == 0 {
		return fmt.Errorf("端口号不能为0")
	}
	if req.ChargeCommand != dny_protocol.ChargeCommandStart &&
		req.ChargeCommand != dny_protocol.ChargeCommandStop &&
		req.ChargeCommand != dny_protocol.ChargeCommandQuery {
		return fmt.Errorf("无效的充电命令: %d", req.ChargeCommand)
	}
	if req.OrderNumber == "" && req.ChargeCommand == dny_protocol.ChargeCommandStart {
		return fmt.Errorf("开始充电时订单编号不能为空")
	}
	return nil
}

// ToProtocolData 将DTO转换为DNY协议数据格式
func (req *ChargeControlRequest) ToProtocolData() []byte {
	// 确保订单编号长度为16字节
	orderBytes := make([]byte, 16)
	if len(req.OrderNumber) > 0 {
		copy(orderBytes, []byte(req.OrderNumber))
	}

	// 构建协议数据 (37字节) - 根据AP3000协议文档完整格式
	data := make([]byte, 37)

	// 费率模式(1字节)
	data[0] = req.RateMode

	// 余额/有效期(4字节，小端序)
	binary.LittleEndian.PutUint32(data[1:5], req.Balance)

	// 端口号(1字节)
	data[5] = req.PortNumber

	// 充电命令(1字节)
	data[6] = req.ChargeCommand

	// 充电时长/电量(2字节，小端序)
	binary.LittleEndian.PutUint16(data[7:9], req.ChargeDuration)

	// 订单编号(16字节)
	copy(data[9:25], orderBytes)

	// 最大充电时长(2字节，小端序)
	binary.LittleEndian.PutUint16(data[25:27], req.MaxChargeDuration)

	// 过载功率(2字节，小端序)
	binary.LittleEndian.PutUint16(data[27:29], req.MaxPower)

	// 二维码灯(1字节)
	data[29] = req.QRCodeLight

	// 扩展字段（根据AP3000协议文档V8.6）
	// 长充模式(1字节) - 0=关闭，1=打开
	data[30] = 0

	// 额外浮充时间(2字节，小端序) - 0=不开启
	binary.LittleEndian.PutUint16(data[31:33], 0)

	// 是否跳过短路检测(1字节) - 2=正常检测短路
	data[33] = 2

	// 不判断用户拔出(1字节) - 0=正常判断拔出
	data[34] = 0

	// 强制带充满自停(1字节) - 0=正常
	data[35] = 0

	// 充满功率(1字节) - 0=关闭充满功率判断
	data[36] = 0

	return data
}

// FromProtocolData 从DNY协议数据解析为DTO
func (resp *ChargeControlResponse) FromProtocolData(data []byte) error {
	if len(data) < 1 {
		return fmt.Errorf("数据长度不足")
	}

	// 响应状态(1字节)
	resp.ResponseStatus = data[0]
	resp.StatusDesc = GetChargeResponseStatusDesc(resp.ResponseStatus)

	// 如果有更多数据，解析订单编号和端口信息
	if len(data) >= 19 {
		// 订单编号(16字节)
		orderBytes := data[1:17]
		resp.OrderNumber = string(orderBytes)

		// 端口号(1字节)
		resp.PortNumber = data[17]

		// 待充端口(2字节，如果有)
		if len(data) >= 21 {
			resp.WaitPorts = binary.LittleEndian.Uint16(data[18:20])
		}
	}

	return nil
}

// GetChargeResponseStatusDesc 获取充电响应状态描述
func GetChargeResponseStatusDesc(status byte) string {
	switch status {
	case dny_protocol.ChargeResponseSuccess:
		return "执行成功"
	case dny_protocol.ChargeResponseNoCharger:
		return "端口未插充电器"
	case dny_protocol.ChargeResponseSameState:
		return "端口状态和充电命令相同"
	case dny_protocol.ChargeResponsePortError:
		return "端口故障"
	case dny_protocol.ChargeResponseNoSuchPort:
		return "无此端口号"
	case dny_protocol.ChargeResponseMultipleWaitPorts:
		return "有多个待充端口"
	case dny_protocol.ChargeResponseOverPower:
		return "多路设备功率超标"
	case dny_protocol.ChargeResponseStorageError:
		return "存储器损坏"
	case dny_protocol.ChargeResponseRelayFault:
		return "继电器坏或保险丝断"
	case dny_protocol.ChargeResponseRelayStuck:
		return "继电器粘连"
	case dny_protocol.ChargeResponseShortCircuit:
		return "负载短路"
	case dny_protocol.ChargeResponseSmokeAlarm:
		return "烟感报警"
	case dny_protocol.ChargeResponseOverVoltage:
		return "过压"
	case dny_protocol.ChargeResponseUnderVoltage:
		return "欠压"
	case dny_protocol.ChargeResponseNoResponse:
		return "未响应"
	default:
		return "未知状态"
	}
}
