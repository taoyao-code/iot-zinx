package handlers

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"
)

// TCPDataLogger 用于记录和分析TCP数据
type TCPDataLogger struct {
	// 日志文件
	logFile     *os.File
	logFilePath string

	// 互斥锁，保证线程安全
	mu sync.Mutex

	// 是否启用详细解析
	enableParsing bool
}

// NewTCPDataLogger 创建一个新的TCP数据记录器
func NewTCPDataLogger(logDir string, enableParsing bool) (*TCPDataLogger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 创建日志文件
	timestamp := time.Now().Format("20060102_150405")
	logFilePath := filepath.Join(logDir, fmt.Sprintf("tcp_data_%s.log", timestamp))
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %v", err)
	}

	logger := &TCPDataLogger{
		logFile:       logFile,
		logFilePath:   logFilePath,
		enableParsing: enableParsing,
	}

	// 写入日志头
	logger.writeHeader()

	return logger, nil
}

// writeHeader 写入日志头信息
func (l *TCPDataLogger) writeHeader() {
	header := "============= TCP数据记录 =============\n"
	header += fmt.Sprintf("开始时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	header += "=======================================\n\n"
	_, _ = l.logFile.WriteString(header)
}

// LogTCPData 记录TCP数据
func (l *TCPDataLogger) LogTCPData(data []byte, direction string, remoteAddr string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取当前时间
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 记录基本信息
	logEntry := fmt.Sprintf("[%s] %s %s\n", timestamp, direction, remoteAddr)
	logEntry += fmt.Sprintf("数据(HEX): %s\n", hex.EncodeToString(data))

	// 如果启用解析，尝试解析DNY协议
	// 🔧 使用统一的DNY协议检查接口
	if l.enableParsing && pkg.Protocol.IsDNYProtocolData(data) {
		result, err := pkg.Protocol.ParseDNYData(data)
		if err == nil {
			logEntry += "解析结果:\n"
			logEntry += result.String() + "\n"
		} else {
			logEntry += fmt.Sprintf("解析失败: %v\n", err)
		}
	}

	logEntry += "---------------------------------------\n"

	// 写入日志文件
	_, _ = l.logFile.WriteString(logEntry)
	_ = l.logFile.Sync() // 确保数据写入磁盘
}

// Close 关闭日志记录器
func (l *TCPDataLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 写入日志尾
	footer := "\n============= 记录结束 =============\n"
	footer += fmt.Sprintf("结束时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	footer += "=======================================\n"
	_, _ = l.logFile.WriteString(footer)

	// 关闭日志文件
	return l.logFile.Close()
}

// GetLogFilePath 获取日志文件路径
func (l *TCPDataLogger) GetLogFilePath() string {
	return l.logFilePath
}

// LogMessage 记录一条消息
func (l *TCPDataLogger) LogMessage(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取当前时间
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 记录消息
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, message)
	logEntry += "---------------------------------------\n"

	// 写入日志文件
	_, _ = l.logFile.WriteString(logEntry)
	_ = l.logFile.Sync() // 确保数据写入磁盘
}

// CopyTo 将日志内容复制到指定的Writer
func (l *TCPDataLogger) CopyTo(writer io.Writer) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 确保所有数据都写入文件
	_ = l.logFile.Sync()

	// 将文件指针移动到开头
	_, err := l.logFile.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("移动文件指针失败: %v", err)
	}

	// 复制日志内容
	_, err = io.Copy(writer, l.logFile)
	if err != nil {
		return fmt.Errorf("复制日志内容失败: %v", err)
	}

	// 将文件指针移动到末尾
	_, err = l.logFile.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("移动文件指针失败: %v", err)
	}

	return nil
}

// ParseHexString 解析十六进制字符串并记录
func (l *TCPDataLogger) ParseHexString(hexStr string, description string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 获取当前时间
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 记录基本信息
	logEntry := fmt.Sprintf("[%s] 手动解析: %s\n", timestamp, description)
	logEntry += fmt.Sprintf("数据(HEX): %s\n", hexStr)

	// 尝试解析DNY协议
	result, err := pkg.Protocol.ParseDNYHexString(hexStr)
	if err == nil {
		logEntry += "解析结果:\n"
		logEntry += result.String() + "\n"
	} else {
		logEntry += fmt.Sprintf("解析失败: %v\n", err)
	}

	logEntry += "---------------------------------------\n"

	// 写入日志文件
	_, _ = l.logFile.WriteString(logEntry)
	_ = l.logFile.Sync() // 确保数据写入磁盘
}
