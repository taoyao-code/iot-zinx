package service

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ICCIDInterceptor ICCID消息拦截器
// 负责处理首次连接时设备发送的ICCID字符串
type ICCIDInterceptor struct{}

// Intercept 拦截处理ICCID消息
func (i *ICCIDInterceptor) Intercept(conn ziface.IConnection, data []byte) bool {
	// 优先级1：检查是否已处理过ICCID（避免重复处理）
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		return false
	}

	// 尝试将数据解析为ASCII字符串
	hexStr := hex.EncodeToString(data)
	asciiStr := strings.TrimSpace(string(data))

	// 优先级2：检查是否为ICCID特征字符串
	if len(asciiStr) >= 10 && len(asciiStr) <= 22 {
		if strings.HasPrefix(asciiStr, "89") {
			// 设置连接ICCID属性
			conn.SetProperty(constants.PropKeyICCID, asciiStr)
			// 设置直连模式属性
			conn.SetProperty(constants.PropKeyDirectMode, true)

			logger.WithFields(logrus.Fields{
				"data":       asciiStr,
				"msgType":    "ICCID",
				"directMode": true,
			}).Info("检测到ICCID特殊消息")
			return true
		}
	}

	// 优先级3：检查十六进制字符串是否可能为ICCID编码
	if len(hexStr) >= 20 && len(hexStr) <= 44 {
		// 尝试作为十六进制解码
		if asciiBuf, err := hex.DecodeString(hexStr); err == nil {
			asciiFromHex := strings.TrimSpace(string(asciiBuf))
			if len(asciiFromHex) >= 10 && len(asciiFromHex) <= 22 && strings.HasPrefix(asciiFromHex, "89") {
				// 设置连接ICCID属性
				conn.SetProperty(constants.PropKeyICCID, asciiFromHex)
				// 设置直连模式属性
				conn.SetProperty(constants.PropKeyDirectMode, true)

				logger.WithFields(logrus.Fields{
					"data":       fmt.Sprintf("%s -> %s", hexStr, asciiFromHex),
					"msgType":    "ICCID(HEX)",
					"directMode": true,
				}).Info("检测到编码为HEX的ICCID特殊消息")
				return true
			}
		}
	}

	return false
}
