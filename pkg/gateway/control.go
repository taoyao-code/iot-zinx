package gateway

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// SendLocationCommand 发送设备定位命令（0x96）
func (g *DeviceGateway) SendLocationCommand(deviceID string, locateTime int) error {
	locationDuration := byte(locateTime)

	logger.WithFields(logrus.Fields{
		"deviceID":       deviceID,
		"command":        "DEVICE_LOCATE",
		"commandID":      fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
		"locateTime":     locateTime,
		"actualDuration": locationDuration,
		"action":         "PREPARE_SEND",
		"timestamp":      time.Now().Format("2006-01-02 15:04:05"),
	}).Info("🎯 准备发送设备定位命令")

	if err := g.SendCommandToDevice(deviceID, constants.CmdDeviceLocate, []byte{locationDuration}); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":   deviceID,
			"command":    "DEVICE_LOCATE",
			"commandID":  fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
			"locateTime": locateTime,
			"error":      err.Error(),
			"action":     "SEND_FAILED",
			"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
		}).Error("❌ 设备定位命令发送失败")
		return fmt.Errorf("发送定位命令失败: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"deviceID":         deviceID,
		"command":          "DEVICE_LOCATE",
		"commandID":        fmt.Sprintf("0x%02X", constants.CmdDeviceLocate),
		"locateTime":       locateTime,
		"duration":         locationDuration,
		"action":           "SEND_SUCCESS",
		"expectedBehavior": "设备将播放语音并闪灯",
		"timestamp":        time.Now().Format("2006-01-02 15:04:05"),
	}).Info("🔊 设备定位命令发送成功")
	return nil
}
