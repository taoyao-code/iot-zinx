package protocol

import (
	"fmt"
	"strconv"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// SetDNYConnectionProperties 设置DNY消息的连接属性
// 这个函数整合了原来分散在解码器中的属性设置逻辑，减少代码重复
func SetDNYConnectionProperties(conn ziface.IConnection, dnyMsg *dny_protocol.Message, rawData []byte) {
	if conn == nil || dnyMsg == nil {
		return
	}

	// 使用DeviceSession统一管理连接属性
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		// 设置物理ID属性
		physicalID := dnyMsg.GetPhysicalId()
		deviceSession.PhysicalID = strconv.FormatUint(uint64(physicalID), 10)

		// 同步到连接属性
		deviceSession.SyncToConnection(conn)

		logger.WithFields(logrus.Fields{
			"physicalID": physicalID,
			"messageID":  fmt.Sprintf("0x%04X", dnyMsg.MessageId),
			"command":    fmt.Sprintf("0x%02X", dnyMsg.GetMsgID()),
		}).Debug("已设置DNY连接属性")
	}
}

// SetSpecialMessageProperties 设置特殊消息属性
func SetSpecialMessageProperties(conn ziface.IConnection, dnyMsg *dny_protocol.Message, rawData []byte) {
	if conn == nil || dnyMsg == nil {
		return
	}

	// 使用DeviceSession统一管理连接属性
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession == nil {
		return
	}

	switch dnyMsg.GetMsgID() {
	case constants.MsgIDICCID:
		iccid := string(dnyMsg.GetData())
		// ICCID通过DeviceSession管理
		deviceSession.ICCID = iccid
		deviceSession.SyncToConnection(conn)
		logger.WithField("iccid", iccid).Info("已设置ICCID连接属性")

	case constants.MsgIDLinkHeartbeat:
		// 心跳信息通过DeviceSession管理
		deviceSession.UpdateHeartbeat()
		deviceSession.SyncToConnection(conn)
		logger.Info("已设置link心跳连接属性")

	default:
		logger.WithField("msgId", dnyMsg.GetMsgID()).Debug("已设置特殊消息连接属性")
	}
}

// SetErrorProperties 设置错误属性
func SetErrorProperties(conn ziface.IConnection, rawData []byte, err error) {
	if conn == nil {
		return
	}

	// 错误信息通过日志记录，不需要存储在连接属性中
	logger.WithFields(logrus.Fields{
		"dataLen": len(rawData),
		"error":   err.Error(),
	}).Debug("协议解析错误")
}

// setChecksumProperties 设置校验和属性（内部函数）
func setChecksumProperties(conn ziface.IConnection, dnyMsg *dny_protocol.Message, rawData []byte) {
	if len(rawData) < 14 {
		return
	}

	checksumPos := 12 + len(dnyMsg.GetData())
	if checksumPos+1 >= len(rawData) {
		return
	}

	// 从数据中获取校验和
	checksum := uint16(rawData[checksumPos]) | uint16(rawData[checksumPos+1])<<8

	// 使用简单的累加校验和进行验证
	calculatedChecksum := CalculatePacketChecksum(rawData[:checksumPos])
	checksumValid := (calculatedChecksum == checksum)

	// 如果校验和无效，记录详细信息
	if !checksumValid {
		logger.WithFields(logrus.Fields{
			"command":            fmt.Sprintf("0x%02X", uint8(dnyMsg.GetMsgID())),
			"expectedChecksum":   fmt.Sprintf("0x%04X", checksum),
			"calculatedChecksum": fmt.Sprintf("0x%04X", calculatedChecksum),
			"checksumValid":      checksumValid,
		}).Warn("DNY协议校验和验证失败")
	}
}
