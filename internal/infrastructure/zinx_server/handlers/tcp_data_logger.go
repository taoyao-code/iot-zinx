package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// TCPDataLogger TCP数据日志记录器
// 负责记录所有TCP连接的数据传输
type TCPDataLogger struct {
	// 日志目录
	logDir string

	// 文件锁，避免并发写入
	fileMutex sync.Mutex

	// 启用DNY协议解析
	enableParsing bool
}

// NewTCPDataLogger 创建TCP数据日志记录器
func NewTCPDataLogger(logDir string, enableParsing bool) (*TCPDataLogger, error) {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	return &TCPDataLogger{
		logDir:        logDir,
		enableParsing: enableParsing,
	}, nil
}

// LogMessage 记录普通消息
func (l *TCPDataLogger) LogMessage(message string) {
	l.writeToLogFile("system", message)
}

// LogData 记录TCP数据
func (l *TCPDataLogger) LogData(connID uint64, remoteAddr string, data []byte, direction string) {
	// 构建基本日志信息
	timestamp := time.Now().Format(constants.TimeFormatDefault)
	dataHex := fmt.Sprintf("%x", data)

	// 基本日志行
	logLine := fmt.Sprintf("[%s] %s - ConnID: %d, 远程地址: %s\n数据(HEX): %s\n长度: %d\n",
		timestamp, direction, connID, remoteAddr, dataHex, len(data))

	// 附加协议解析信息（如果启用）
	var parseInfo string
	if l.enableParsing && utils.IsDNYProtocolData(data) {
		result, err := protocol.ParseDNYData(data)
		if err == nil && result != nil {
			parseInfo = fmt.Sprintf("DNY协议: PhysicalID=%08X, Command=0x%02X(%s), Length=%d, Data=%s, Checksum=%v\n",
				result.PhysicalID, result.Command, result.CommandName, len(result.Data),
				fmt.Sprintf("%x", result.Data), result.ChecksumValid)
		}
	}

	// 写入日志文件
	l.writeToLogFile(fmt.Sprintf("conn_%d", connID), logLine+parseInfo)
}

// LogHexData 记录十六进制字符串数据
func (l *TCPDataLogger) LogHexData(connID uint64, remoteAddr string, hexStr string, description string) {
	// 构建基本日志信息
	timestamp := time.Now().Format(constants.TimeFormatDefault)

	// 去除非十六进制字符
	cleanHex := make([]byte, 0, len(hexStr))
	for i := 0; i < len(hexStr); i++ {
		char := hexStr[i]
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			cleanHex = append(cleanHex, char)
		}
	}

	// 基本日志行
	logLine := fmt.Sprintf("[%s] %s - ConnID: %d, 远程地址: %s\n十六进制字符串: %s\n长度: %d\n",
		timestamp, description, connID, remoteAddr, string(cleanHex), len(cleanHex))

	// 附加协议解析信息（如果启用）
	var parseInfo string
	result, err := protocol.ParseDNYHexString(hexStr)
	if err == nil && result != nil {
		parseInfo = fmt.Sprintf("DNY协议解析: PhysicalID=0x%08X, Command=0x%02X(%s), Length=%d, Data=%s, Checksum=%v\n",
			result.PhysicalID, result.Command, result.CommandName, len(result.Data),
			fmt.Sprintf("%x", result.Data), result.ChecksumValid)
	}

	// 写入日志文件
	l.writeToLogFile(fmt.Sprintf("conn_%d", connID), logLine+parseInfo)
}

// writeToLogFile 写入日志文件
func (l *TCPDataLogger) writeToLogFile(prefix string, content string) {
	l.fileMutex.Lock()
	defer l.fileMutex.Unlock()

	// 创建当天日志目录
	dateDir := filepath.Join(l.logDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"dir":   dateDir,
		}).Error("创建日志子目录失败")
		return
	}

	// 日志文件路径
	logFilePath := filepath.Join(dateDir, fmt.Sprintf("%s.log", prefix))

	// 打开日志文件（追加模式）
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"filePath": logFilePath,
		}).Error("打开日志文件失败")
		return
	}
	defer file.Close()

	// 写入内容并添加分隔符
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "---------------------------------------------------------\n"

	if _, err := file.WriteString(content); err != nil {
		logger.WithFields(logrus.Fields{
			"error":    err.Error(),
			"filePath": logFilePath,
		}).Error("写入日志文件失败")
	}
}
