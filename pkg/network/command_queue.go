package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"
)

// CommandPriority 命令优先级
type CommandPriority int

const (
	PriorityLow    CommandPriority = 1
	PriorityNormal CommandPriority = 2
	PriorityHigh   CommandPriority = 3
	PriorityUrgent CommandPriority = 4
)

// QueuedCommand 队列中的命令
type QueuedCommand struct {
	ID          string
	ConnID      uint32
	Connection  ziface.IConnection
	MsgID       uint32
	Data        []byte
	Priority    CommandPriority
	CreatedAt   time.Time
	Timeout     time.Duration
	RetryCount  int
	MaxRetries  int
	Callback    func(error)
	Context     context.Context
}

// CommandQueue 命令队列
type CommandQueue struct {
	queues     map[CommandPriority]chan *QueuedCommand
	workers    int
	writer     *TCPWriter
	logger     *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mutex      sync.RWMutex
	stats      *QueueStats
}

// QueueStats 队列统计
type QueueStats struct {
	TotalEnqueued   int64
	TotalProcessed  int64
	TotalFailed     int64
	TotalTimeout    int64
	CurrentPending  int64
	mutex           sync.RWMutex
}

// NewCommandQueue 创建命令队列
func NewCommandQueue(workers int, writer *TCPWriter, logger *logrus.Logger) *CommandQueue {
	ctx, cancel := context.WithCancel(context.Background())
	
	cq := &CommandQueue{
		queues: make(map[CommandPriority]chan *QueuedCommand),
		workers: workers,
		writer:  writer,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		stats:   &QueueStats{},
	}

	// 初始化各优先级队列
	cq.queues[PriorityUrgent] = make(chan *QueuedCommand, 100)
	cq.queues[PriorityHigh] = make(chan *QueuedCommand, 200)
	cq.queues[PriorityNormal] = make(chan *QueuedCommand, 500)
	cq.queues[PriorityLow] = make(chan *QueuedCommand, 1000)

	return cq
}

// Start 启动命令队列
func (cq *CommandQueue) Start() {
	for i := 0; i < cq.workers; i++ {
		cq.wg.Add(1)
		go cq.worker(i)
	}
	
	cq.logger.WithField("workers", cq.workers).Info("🚀 命令队列已启动")
}

// Stop 停止命令队列
func (cq *CommandQueue) Stop() {
	cq.cancel()
	
	// 关闭所有队列
	for _, queue := range cq.queues {
		close(queue)
	}
	
	cq.wg.Wait()
	cq.logger.Info("🛑 命令队列已停止")
}

// EnqueueCommand 入队命令
func (cq *CommandQueue) EnqueueCommand(cmd *QueuedCommand) error {
	if cmd.Context == nil {
		cmd.Context = context.Background()
	}
	
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = time.Now()
	}
	
	if cmd.Timeout == 0 {
		cmd.Timeout = 30 * time.Second // 默认超时时间
	}

	queue, exists := cq.queues[cmd.Priority]
	if !exists {
		return fmt.Errorf("不支持的命令优先级: %d", cmd.Priority)
	}

	select {
	case queue <- cmd:
		cq.updateStats(func(s *QueueStats) {
			s.TotalEnqueued++
			s.CurrentPending++
		})
		
		cq.logger.WithFields(logrus.Fields{
			"cmdID":    cmd.ID,
			"connID":   cmd.ConnID,
			"priority": cmd.Priority,
			"dataSize": len(cmd.Data),
		}).Debug("命令已入队")
		
		return nil
		
	case <-cq.ctx.Done():
		return fmt.Errorf("命令队列已停止")
		
	default:
		return fmt.Errorf("命令队列已满，优先级: %d", cmd.Priority)
	}
}

// worker 工作协程
func (cq *CommandQueue) worker(workerID int) {
	defer cq.wg.Done()
	
	cq.logger.WithField("workerID", workerID).Debug("命令队列工作协程启动")
	
	for {
		cmd := cq.getNextCommand()
		if cmd == nil {
			// 队列已关闭
			break
		}
		
		cq.processCommand(workerID, cmd)
	}
	
	cq.logger.WithField("workerID", workerID).Debug("命令队列工作协程退出")
}

// getNextCommand 获取下一个命令（按优先级）
func (cq *CommandQueue) getNextCommand() *QueuedCommand {
	// 按优先级顺序检查队列
	priorities := []CommandPriority{PriorityUrgent, PriorityHigh, PriorityNormal, PriorityLow}
	
	for {
		for _, priority := range priorities {
			select {
			case cmd := <-cq.queues[priority]:
				if cmd != nil {
					cq.updateStats(func(s *QueueStats) {
						s.CurrentPending--
					})
				}
				return cmd
			default:
				// 该优先级队列为空，检查下一个
			}
		}
		
		// 所有队列都为空，等待或退出
		select {
		case <-cq.ctx.Done():
			return nil
		case <-time.After(10 * time.Millisecond):
			// 短暂等待后重新检查
		}
	}
}

// processCommand 处理命令
func (cq *CommandQueue) processCommand(workerID int, cmd *QueuedCommand) {
	startTime := time.Now()
	
	// 检查超时
	if time.Since(cmd.CreatedAt) > cmd.Timeout {
		cq.updateStats(func(s *QueueStats) {
			s.TotalTimeout++
			s.TotalFailed++
		})
		
		err := fmt.Errorf("命令超时")
		cq.logger.WithFields(logrus.Fields{
			"workerID": workerID,
			"cmdID":    cmd.ID,
			"connID":   cmd.ConnID,
			"timeout":  cmd.Timeout,
			"elapsed":  time.Since(cmd.CreatedAt),
		}).Warn("命令处理超时")
		
		if cmd.Callback != nil {
			cmd.Callback(err)
		}
		return
	}
	
	// 检查连接是否有效
	if cmd.Connection == nil {
		err := fmt.Errorf("连接为空")
		cq.handleCommandError(workerID, cmd, err)
		return
	}
	
	// 执行命令
	var err error
	if cq.writer != nil {
		err = cq.writer.SendBuffMsgWithRetry(cmd.Connection, cmd.MsgID, cmd.Data)
	} else {
		err = cmd.Connection.SendBuffMsg(cmd.MsgID, cmd.Data)
	}
	
	duration := time.Since(startTime)
	
	if err != nil {
		cq.handleCommandError(workerID, cmd, err)
	} else {
		cq.updateStats(func(s *QueueStats) {
			s.TotalProcessed++
		})
		
		cq.logger.WithFields(logrus.Fields{
			"workerID": workerID,
			"cmdID":    cmd.ID,
			"connID":   cmd.ConnID,
			"duration": duration,
			"dataSize": len(cmd.Data),
		}).Debug("命令处理成功")
		
		if cmd.Callback != nil {
			cmd.Callback(nil)
		}
	}
}

// handleCommandError 处理命令错误
func (cq *CommandQueue) handleCommandError(workerID int, cmd *QueuedCommand, err error) {
	cq.updateStats(func(s *QueueStats) {
		s.TotalFailed++
	})
	
	cq.logger.WithFields(logrus.Fields{
		"workerID": workerID,
		"cmdID":    cmd.ID,
		"connID":   cmd.ConnID,
		"error":    err.Error(),
		"retries":  cmd.RetryCount,
	}).Error("命令处理失败")
	
	if cmd.Callback != nil {
		cmd.Callback(err)
	}
}

// updateStats 更新统计信息
func (cq *CommandQueue) updateStats(fn func(*QueueStats)) {
	cq.stats.mutex.Lock()
	fn(cq.stats)
	cq.stats.mutex.Unlock()
}

// GetStats 获取队列统计信息
func (cq *CommandQueue) GetStats() QueueStats {
	cq.stats.mutex.RLock()
	defer cq.stats.mutex.RUnlock()
	
	return QueueStats{
		TotalEnqueued:  cq.stats.TotalEnqueued,
		TotalProcessed: cq.stats.TotalProcessed,
		TotalFailed:    cq.stats.TotalFailed,
		TotalTimeout:   cq.stats.TotalTimeout,
		CurrentPending: cq.stats.CurrentPending,
	}
}

// GetQueueLengths 获取各优先级队列长度
func (cq *CommandQueue) GetQueueLengths() map[CommandPriority]int {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()
	
	lengths := make(map[CommandPriority]int)
	for priority, queue := range cq.queues {
		lengths[priority] = len(queue)
	}
	
	return lengths
}

// LogStats 记录统计信息
func (cq *CommandQueue) LogStats() {
	stats := cq.GetStats()
	lengths := cq.GetQueueLengths()
	
	var successRate float64
	if stats.TotalEnqueued > 0 {
		successRate = float64(stats.TotalProcessed) / float64(stats.TotalEnqueued) * 100
	}
	
	cq.logger.WithFields(logrus.Fields{
		"totalEnqueued":  stats.TotalEnqueued,
		"totalProcessed": stats.TotalProcessed,
		"totalFailed":    stats.TotalFailed,
		"totalTimeout":   stats.TotalTimeout,
		"currentPending": stats.CurrentPending,
		"successRate":    fmt.Sprintf("%.2f%%", successRate),
		"queueLengths":   lengths,
	}).Info("📊 命令队列统计报告")
}