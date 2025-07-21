package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyRotator 按日期分割的日志轮转器
type DailyRotator struct {
	// 基础配置
	BaseDir    string // 日志基础目录
	FilePrefix string // 文件前缀，如 "gateway"
	MaxAge     int    // 保留天数
	Compress   bool   // 是否压缩旧文件
	LocalTime  bool   // 是否使用本地时间

	// 内部状态
	mu          sync.Mutex
	currentFile *os.File
	currentDate string

	// 清理配置
	cleanupEnabled bool
	lastCleanup    time.Time
}

// NewDailyRotator 创建新的日期轮转器
func NewDailyRotator(baseDir, filePrefix string, maxAge int) *DailyRotator {
	return &DailyRotator{
		BaseDir:        baseDir,
		FilePrefix:     filePrefix,
		MaxAge:         maxAge,
		Compress:       true,
		LocalTime:      true,
		cleanupEnabled: true,
	}
}

// Write 实现 io.Writer 接口
func (r *DailyRotator) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否需要轮转
	if err := r.checkRotation(); err != nil {
		return 0, err
	}

	// 写入数据
	if r.currentFile == nil {
		return 0, fmt.Errorf("日志文件未打开")
	}

	return r.currentFile.Write(p)
}

// checkRotation 检查是否需要轮转
func (r *DailyRotator) checkRotation() error {
	now := time.Now()
	if r.LocalTime {
		now = now.Local()
	} else {
		now = now.UTC()
	}

	currentDate := now.Format("2006-01-02")

	// 如果日期没有变化且文件已打开，无需轮转
	if r.currentDate == currentDate && r.currentFile != nil {
		return nil
	}

	// 关闭当前文件
	if r.currentFile != nil {
		r.currentFile.Close()
		r.currentFile = nil
	}

	// 创建新文件
	if err := r.openNewFile(currentDate); err != nil {
		return err
	}

	r.currentDate = currentDate

	// 执行清理（每天最多一次）
	if r.cleanupEnabled && now.Sub(r.lastCleanup) > 23*time.Hour {
		go r.cleanup()
		r.lastCleanup = now
	}

	return nil
}

// openNewFile 打开新的日志文件
func (r *DailyRotator) openNewFile(date string) error {
	// 确保目录存在
	if err := os.MkdirAll(r.BaseDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 生成文件名：gateway-2024-01-15.log
	filename := fmt.Sprintf("%s-%s.log", r.FilePrefix, date)
	filepath := filepath.Join(r.BaseDir, filename)

	// 打开文件（追加模式）
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	r.currentFile = file
	return nil
}

// cleanup 清理过期的日志文件
func (r *DailyRotator) cleanup() {
	if r.MaxAge <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -r.MaxAge)

	// 扫描日志目录
	entries, err := os.ReadDir(r.BaseDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 检查是否是我们的日志文件
		name := entry.Name()
		if !r.isOurLogFile(name) {
			continue
		}

		// 获取文件信息
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 检查是否过期
		if info.ModTime().Before(cutoff) {
			filepath := filepath.Join(r.BaseDir, name)

			// 如果启用压缩，先压缩再删除
			if r.Compress && !r.isCompressed(name) {
				if err := r.compressFile(filepath); err == nil {
					os.Remove(filepath) // 删除原文件
				}
			} else if r.isCompressed(name) || !r.Compress {
				os.Remove(filepath) // 直接删除
			}
		}
	}
}

// isOurLogFile 检查是否是我们的日志文件
func (r *DailyRotator) isOurLogFile(filename string) bool {
	prefix := r.FilePrefix + "-"
	return len(filename) > len(prefix) && filename[:len(prefix)] == prefix
}

// isCompressed 检查文件是否已压缩
func (r *DailyRotator) isCompressed(filename string) bool {
	return filepath.Ext(filename) == ".gz"
}

// compressFile 压缩文件
func (r *DailyRotator) compressFile(filepath string) error {
	// 这里可以实现gzip压缩
	// 为了简化，暂时跳过压缩功能
	return nil
}

// Close 关闭轮转器
func (r *DailyRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentFile != nil {
		err := r.currentFile.Close()
		r.currentFile = nil
		return err
	}
	return nil
}

// GetCurrentFilePath 获取当前日志文件路径
func (r *DailyRotator) GetCurrentFilePath() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.currentDate == "" {
		now := time.Now()
		if r.LocalTime {
			now = now.Local()
		}
		r.currentDate = now.Format("2006-01-02")
	}

	filename := fmt.Sprintf("%s-%s.log", r.FilePrefix, r.currentDate)
	return filepath.Join(r.BaseDir, filename)
}

// MultiWriter 创建多路输出器，同时写入控制台和日志文件
func NewMultiWriter(rotator *DailyRotator, enableConsole bool) io.Writer {
	writers := []io.Writer{rotator}

	if enableConsole {
		writers = append(writers, os.Stdout)
	}

	return io.MultiWriter(writers...)
}
