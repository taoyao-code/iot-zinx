package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// writeTimeBytes 写入时间字节 (6字节: 年月日时分秒)
func writeTimeBytes(buf *bytes.Buffer, t time.Time) {
	year := uint16(t.Year())
	month := uint8(t.Month())
	day := uint8(t.Day())
	hour := uint8(t.Hour())
	minute := uint8(t.Minute())
	second := uint8(t.Second())

	if err := binary.Write(buf, binary.LittleEndian, year); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}
	buf.WriteByte(month)
	buf.WriteByte(day)
	buf.WriteByte(hour)
	buf.WriteByte(minute)
	buf.WriteByte(second)
}

// readTimeBytes 读取时间字节 (6字节: 年月日时分秒)
func readTimeBytes(data []byte) time.Time {
	if len(data) < 6 {
		return time.Now()
	}

	year := binary.LittleEndian.Uint16(data[0:2])
	month := data[2]
	day := data[3]
	hour := data[4]
	minute := data[5]
	second := uint8(0) // 6字节格式中没有秒数字段，设为0

	return time.Date(int(year), time.Month(month), int(day),
		int(hour), int(minute), int(second), 0, time.Local)
}

// GetMessageTypeName 获取消息类型的可读名称
func GetMessageTypeName(msgType MessageType) string {
	switch msgType {
	case MsgTypeOldHeartbeat:
		return "旧版设备心跳包(01指令)"
	case MsgTypeSwipeCard:
		return "刷卡操作(02指令)"
	case MsgTypeSettlement:
		return "结算消费信息上传(03指令)"
	case MsgTypeOrderConfirm:
		return "充电端口订单确认(04指令)"
	case MsgTypeExtendedCommand:
		return "扩展命令类型(05指令)"
	case MsgTypePowerHeartbeat:
		return "端口充电时功率心跳包(06指令)"
	case MsgTypeMainHeartbeat:
		return "主机状态心跳包(11指令)"
	case MsgTypeMainGetServerTime:
		return "主机获取服务器时间(12指令)"
	case MsgTypeDeviceRegister:
		return "设备注册包(20指令)"
	case MsgTypeHeartbeat:
		return "设备心跳包(21指令)"
	case MsgTypeServerTimeRequest:
		return "设备获取服务器时间(22指令)"
	case MsgTypeServerQuery:
		return "服务器查询设备联网状态(81指令)"
	case MsgTypeChargeControl:
		return "服务器开始、停止充电操作(82指令)"
	case MsgTypeModifyCharge:
		return "服务器修改充电时长/电量(8A指令)"

	// 扩展消息类型
	case MsgTypeExtHeartbeat1:
		return "扩展心跳包类型1(87指令)"
	case MsgTypeExtHeartbeat2:
		return "扩展心跳包类型2(88指令)"
	case MsgTypeExtHeartbeat3:
		return "扩展心跳包类型3(89指令)"
	case MsgTypeExtHeartbeat4:
		return "扩展心跳包类型4(A0指令)"
	case MsgTypeExtHeartbeat5:
		return "扩展心跳包类型5(8B指令)"
	case MsgTypeExtHeartbeat6:
		return "扩展心跳包类型6(8C指令)"
	case MsgTypeExtHeartbeat7:
		return "扩展心跳包类型7(8D指令)"
	case MsgTypeExtHeartbeat8:
		return "扩展心跳包类型8(8E指令)"
	case MsgTypeExtCommand1:
		return "扩展命令类型1(8F指令)"
	case MsgTypeExtStatus1:
		return "扩展状态类型1(90指令)"
	case MsgTypeExtStatus2:
		return "扩展状态类型2(91指令)"
	case MsgTypeExtStatus3:
		return "扩展状态类型3(92指令)"
	case MsgTypeExtStatus4:
		return "扩展状态类型4(93指令)"
	case MsgTypeExtStatus5:
		return "扩展状态类型5(94指令)"
	case MsgTypeExtStatus6:
		return "扩展状态类型6(95指令)"
	case MsgTypeDeviceLocate:
		return "声光寻找设备功能(96指令)"
	case MsgTypeExtCommand2:
		return "扩展命令类型2(97指令)"
	case MsgTypeExtStatus8:
		return "扩展状态类型8(98指令)"
	case MsgTypeExtStatus9:
		return "扩展状态类型9(99指令)"
	case MsgTypeExtStatus10:
		return "扩展状态类型10(9A指令)"
	case MsgTypeExtCommand3:
		return "扩展命令类型3(9B指令)"
	case MsgTypeExtStatus11:
		return "扩展状态类型11(A1指令)"
	case MsgTypeExtStatus12:
		return "扩展状态类型12(A2指令)"
	case MsgTypeExtStatus13:
		return "扩展状态类型13(A3指令)"
	case MsgTypeExtStatus14:
		return "扩展状态类型14(A4指令)"
	case MsgTypeExtStatus15:
		return "扩展状态类型15(A6指令)"
	case MsgTypeExtStatus16:
		return "扩展状态类型16(A7指令)"
	case MsgTypeExtStatus17:
		return "扩展状态类型17(A8指令)"
	case MsgTypeExtStatus18:
		return "扩展状态类型18(A9指令)"
	case MsgTypeExtCommand4:
		return "扩展命令类型4(AA指令)"
	case MsgTypeExtStatus19:
		return "扩展状态类型19(AB指令)"
	case MsgTypeExtStatus20:
		return "扩展状态类型20(AC指令)"

	default:
		return fmt.Sprintf("未知类型(0x%02X)", uint8(msgType))
	}
}
