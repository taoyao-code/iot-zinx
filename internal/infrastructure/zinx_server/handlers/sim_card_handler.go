package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// SimCardHandler 处理SIM卡号上报 (命令ID: 0xFF01)
type SimCardHandler struct {
	znet.BaseRouter
}

// Handle 处理SIM卡号上报
func (h *SimCardHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 确保数据是有效的SIM卡号 (支持标准ICCID长度范围: 19-25字节)
	if len(data) >= 19 && len(data) <= 25 && protocol.IsAllDigits(data) {
		// 存储SIM卡号到连接属性
		iccidStr := string(data)
		conn.SetProperty(constants.PropKeyICCID, iccidStr)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"iccid":      iccidStr,
			"dataLen":    len(data),
		}).Info("收到SIM卡号数据")

		// 更新心跳时间
		now := time.Now()
		conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
		conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05"))

		// 更新设备监控
		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"data":       string(data),
		}).Warn("收到无效的SIM卡号数据")
	}
}
