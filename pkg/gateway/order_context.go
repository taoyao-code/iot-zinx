package gateway

import "strconv"

// OrderContext 当前订单上下文（用于仅更新0x82时回填）
type OrderContext struct {
	OrderNo string
	Mode    uint8  // 0=计时,1=包月,2=计量,3=计次
	Value   uint16 // 时长(秒)或电量(0.1度)
	Balance uint32 // 余额/有效期(4B)
}

func (g *DeviceGateway) makeOrderCtxKey(deviceID string, protocolPort int) string {
	return deviceID + "|" + strconv.Itoa(protocolPort)
}
