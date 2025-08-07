package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// IMonitorAggregator 监控数据聚合器接口
type IMonitorAggregator interface {
	// === 数据聚合 ===
	AggregateMetrics(interval time.Duration) (*AggregatedMetrics, error)
	GetAggregatedMetrics(timeRange TimeRange) (*AggregatedMetrics, error)
	GetTrendData(metric string, timeRange TimeRange) (*TrendData, error)

	// === 基础分析（简化版） ===
	// 删除复杂的实时分析功能，保留基础监控

	// === 基础报告（简化版） ===
	// 删除复杂的报告生成功能，保留基础指标导出

	// === 管理操作 ===
	Start() error
	Stop() error
	SetAggregationInterval(interval time.Duration)
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// AggregatedMetrics 聚合指标
type AggregatedMetrics struct {
	TimeRange          TimeRange                          `json:"time_range"`
	ConnectionMetrics  *AggregatedConnectionMetrics       `json:"connection_metrics"`
	DeviceMetrics      *AggregatedDeviceMetrics           `json:"device_metrics"`
	SystemMetrics      *AggregatedSystemMetrics           `json:"system_metrics"`
	PerformanceMetrics *AggregatedPerformanceMetrics      `json:"performance_metrics"`
	CustomMetrics      map[string]*AggregatedCustomMetric `json:"custom_metrics"`
	GeneratedAt        time.Time                          `json:"generated_at"`
}

// AggregatedConnectionMetrics 聚合连接指标
type AggregatedConnectionMetrics struct {
	TotalConnections   MetricSummary `json:"total_connections"`
	ActiveConnections  MetricSummary `json:"active_connections"`
	ConnectionDuration MetricSummary `json:"connection_duration"`
	DataTransferred    MetricSummary `json:"data_transferred"`
	ErrorRate          MetricSummary `json:"error_rate"`
}

// AggregatedDeviceMetrics 聚合设备指标
type AggregatedDeviceMetrics struct {
	TotalDevices       MetricSummary `json:"total_devices"`
	OnlineDevices      MetricSummary `json:"online_devices"`
	DeviceUptime       MetricSummary `json:"device_uptime"`
	HeartbeatFrequency MetricSummary `json:"heartbeat_frequency"`
	RegistrationRate   MetricSummary `json:"registration_rate"`
}

// AggregatedSystemMetrics 聚合系统指标
type AggregatedSystemMetrics struct {
	CPUUsage       MetricSummary `json:"cpu_usage"`
	MemoryUsage    MetricSummary `json:"memory_usage"`
	GoroutineCount MetricSummary `json:"goroutine_count"`
	GCPerformance  MetricSummary `json:"gc_performance"`
}

// AggregatedPerformanceMetrics 聚合性能指标
type AggregatedPerformanceMetrics struct {
	Latency      MetricSummary `json:"latency"`
	Throughput   MetricSummary `json:"throughput"`
	ErrorRate    MetricSummary `json:"error_rate"`
	Availability MetricSummary `json:"availability"`
}

// AggregatedCustomMetric 聚合自定义指标
type AggregatedCustomMetric struct {
	Name    string            `json:"name"`
	Summary MetricSummary     `json:"summary"`
	Tags    map[string]string `json:"tags"`
}

// MetricSummary 指标摘要
type MetricSummary struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Sum    float64 `json:"sum"`
	Count  int64   `json:"count"`
	P50    float64 `json:"p50"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	StdDev float64 `json:"std_dev"`
}

// TrendData 趋势数据
type TrendData struct {
	Metric     string           `json:"metric"`
	TimeRange  TimeRange        `json:"time_range"`
	DataPoints []TrendDataPoint `json:"data_points"`
	Trend      TrendDirection   `json:"trend"`
	ChangeRate float64          `json:"change_rate"`
}

// TrendDataPoint 趋势数据点
type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendUp   TrendDirection = "up"
	TrendDown TrendDirection = "down"
	TrendFlat TrendDirection = "flat"
)

// RealTimeAnalysis 实时分析结果
type RealTimeAnalysis struct {
	Timestamp       time.Time          `json:"timestamp"`
	OverallHealth   HealthLevel        `json:"overall_health"`
	CriticalIssues  []CriticalIssue    `json:"critical_issues"`
	Recommendations []Recommendation   `json:"recommendations"`
	KeyMetrics      map[string]float64 `json:"key_metrics"`
	Alerts          []Alert            `json:"alerts"`
}

// HealthLevel 健康等级
type HealthLevel string

const (
	HealthExcellent HealthLevel = "excellent"
	HealthGood      HealthLevel = "good"
	HealthWarning   HealthLevel = "warning"
	HealthCritical  HealthLevel = "critical"
)

// CriticalIssue 关键问题
type CriticalIssue struct {
	Type               string    `json:"type"`
	Description        string    `json:"description"`
	Severity           string    `json:"severity"`
	DetectedAt         time.Time `json:"detected_at"`
	AffectedComponents []string  `json:"affected_components"`
}

// Recommendation 建议
type Recommendation struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Action      string `json:"action"`
}

// Anomaly 异常
type Anomaly struct {
	ID          string    `json:"id"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Expected    float64   `json:"expected"`
	Deviation   float64   `json:"deviation"`
	Severity    string    `json:"severity"`
	DetectedAt  time.Time `json:"detected_at"`
	Description string    `json:"description"`
}

// HealthScore 健康评分
type HealthScore struct {
	Overall     float64            `json:"overall"`
	Components  map[string]float64 `json:"components"`
	Timestamp   time.Time          `json:"timestamp"`
	Level       HealthLevel        `json:"level"`
	Issues      []string           `json:"issues"`
	Suggestions []string           `json:"suggestions"`
}

// ReportType 报告类型
type ReportType string

const (
	ReportTypeDaily   ReportType = "daily"
	ReportTypeWeekly  ReportType = "weekly"
	ReportTypeMonthly ReportType = "monthly"
	ReportTypeCustom  ReportType = "custom"
)

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatXML  ExportFormat = "xml"
)

// MonitorReport 监控报告
type MonitorReport struct {
	Type        ReportType         `json:"type"`
	TimeRange   TimeRange          `json:"time_range"`
	Summary     *ReportSummary     `json:"summary"`
	Metrics     *AggregatedMetrics `json:"metrics"`
	Trends      []TrendData        `json:"trends"`
	Alerts      []Alert            `json:"alerts"`
	Anomalies   []Anomaly          `json:"anomalies"`
	GeneratedAt time.Time          `json:"generated_at"`
	GeneratedBy string             `json:"generated_by"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalConnections int64   `json:"total_connections"`
	TotalDevices     int64   `json:"total_devices"`
	AverageUptime    float64 `json:"average_uptime"`
	ErrorRate        float64 `json:"error_rate"`
	PerformanceScore float64 `json:"performance_score"`
	HealthScore      float64 `json:"health_score"`
	CriticalAlerts   int64   `json:"critical_alerts"`
	ResolvedIssues   int64   `json:"resolved_issues"`
}

// MonitorAggregator 监控数据聚合器实现
type MonitorAggregator struct {
	// === 核心组件 ===
	monitor *UnifiedMonitor

	// === 配置 ===
	aggregationInterval time.Duration
	retentionPeriod     time.Duration

	// === 聚合数据存储 ===
	aggregatedData sync.Map // timestamp -> *AggregatedMetrics
	trendData      sync.Map // metric -> []TrendDataPoint
	anomalies      sync.Map // timestamp -> []Anomaly

	// === 控制管理 ===
	running  bool
	stopChan chan struct{}
	mutex    sync.RWMutex
}

// NewMonitorAggregator 创建监控数据聚合器
func NewMonitorAggregator(monitor *UnifiedMonitor) *MonitorAggregator {
	return &MonitorAggregator{
		monitor:             monitor,
		aggregationInterval: 5 * time.Minute,
		retentionPeriod:     24 * time.Hour,
		stopChan:            make(chan struct{}),
		running:             false,
	}
}

// === 管理操作实现 ===

// Start 启动聚合器
func (a *MonitorAggregator) Start() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.running {
		return fmt.Errorf("监控数据聚合器已在运行")
	}

	a.running = true

	// 启动聚合协程
	go a.aggregationRoutine()

	// 启动清理协程（删除异常检测协程，简化监控）
	go a.cleanupRoutine()

	logger.Info("监控数据聚合器启动成功")
	return nil
}

// Stop 停止聚合器
func (a *MonitorAggregator) Stop() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.running {
		return fmt.Errorf("监控数据聚合器未在运行")
	}

	a.running = false
	close(a.stopChan)

	logger.Info("监控数据聚合器停止成功")
	return nil
}

// SetAggregationInterval 设置聚合间隔
func (a *MonitorAggregator) SetAggregationInterval(interval time.Duration) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.aggregationInterval = interval
}

// === 数据聚合实现 ===

// AggregateMetrics 聚合指标数据
func (a *MonitorAggregator) AggregateMetrics(interval time.Duration) (*AggregatedMetrics, error) {
	now := time.Now()
	startTime := now.Add(-interval)

	// 获取原始指标数据
	allMetrics := a.monitor.GetAllMetrics()

	// 聚合连接指标
	connMetrics := a.aggregateConnectionMetrics(allMetrics.ConnectionMetrics)

	// 聚合设备指标
	deviceMetrics := a.aggregateDeviceMetrics(allMetrics.DeviceMetrics)

	// 聚合系统指标
	systemMetrics := a.aggregateSystemMetrics(allMetrics.SystemMetrics)

	// 聚合性能指标
	perfStats := a.monitor.GetPerformanceStats()
	perfMetrics := a.aggregatePerformanceMetrics(perfStats)

	// 聚合自定义指标
	customMetrics := a.aggregateCustomMetrics(allMetrics.CustomMetrics)

	aggregated := &AggregatedMetrics{
		TimeRange: TimeRange{
			Start: startTime,
			End:   now,
		},
		ConnectionMetrics:  connMetrics,
		DeviceMetrics:      deviceMetrics,
		SystemMetrics:      systemMetrics,
		PerformanceMetrics: perfMetrics,
		CustomMetrics:      customMetrics,
		GeneratedAt:        now,
	}

	// 存储聚合数据
	a.aggregatedData.Store(now.Unix(), aggregated)

	return aggregated, nil
}

// GetAggregatedMetrics 获取指定时间范围的聚合指标
func (a *MonitorAggregator) GetAggregatedMetrics(timeRange TimeRange) (*AggregatedMetrics, error) {
	// 查找最接近的聚合数据
	var closestMetrics *AggregatedMetrics
	var closestTime int64

	a.aggregatedData.Range(func(key, value interface{}) bool {
		timestamp := key.(int64)
		metrics := value.(*AggregatedMetrics)

		if timestamp >= timeRange.Start.Unix() && timestamp <= timeRange.End.Unix() {
			if closestMetrics == nil || abs(timestamp-timeRange.End.Unix()) < abs(closestTime-timeRange.End.Unix()) {
				closestMetrics = metrics
				closestTime = timestamp
			}
		}
		return true
	})

	if closestMetrics == nil {
		return nil, fmt.Errorf("未找到指定时间范围的聚合数据")
	}

	return closestMetrics, nil
}

// GetTrendData 获取趋势数据
func (a *MonitorAggregator) GetTrendData(metric string, timeRange TimeRange) (*TrendData, error) {
	if trendInterface, exists := a.trendData.Load(metric); exists {
		allPoints := trendInterface.([]TrendDataPoint)

		// 过滤时间范围内的数据点
		var filteredPoints []TrendDataPoint
		for _, point := range allPoints {
			if point.Timestamp.After(timeRange.Start) && point.Timestamp.Before(timeRange.End) {
				filteredPoints = append(filteredPoints, point)
			}
		}

		if len(filteredPoints) == 0 {
			return nil, fmt.Errorf("指定时间范围内没有趋势数据")
		}

		// 计算趋势方向和变化率
		trend, changeRate := a.calculateTrend(filteredPoints)

		return &TrendData{
			Metric:     metric,
			TimeRange:  timeRange,
			DataPoints: filteredPoints,
			Trend:      trend,
			ChangeRate: changeRate,
		}, nil
	}

	return nil, fmt.Errorf("未找到指标的趋势数据: %s", metric)
}

// === 基础分析实现（简化版） ===
// 删除复杂的实时分析功能，保留基础监控

// 异常检测功能已删除，简化监控架构

// 健康评分计算功能已删除，简化监控架构

// === 报告生成实现 ===

// GenerateReport 生成监控报告
func (a *MonitorAggregator) GenerateReport(reportType ReportType, timeRange TimeRange) (*MonitorReport, error) {
	now := time.Now()

	// 获取聚合指标
	metrics, err := a.GetAggregatedMetrics(timeRange)
	if err != nil {
		// 如果没有聚合数据，生成当前数据
		metrics, err = a.AggregateMetrics(timeRange.End.Sub(timeRange.Start))
		if err != nil {
			return nil, fmt.Errorf("生成报告失败: %v", err)
		}
	}

	// 获取趋势数据
	var trends []TrendData
	trendMetrics := []string{"active_connections", "online_devices", "error_rate", "average_latency"}
	for _, metric := range trendMetrics {
		if trend, err := a.GetTrendData(metric, timeRange); err == nil {
			trends = append(trends, *trend)
		}
	}

	// 获取告警数据
	alerts := a.monitor.GetActiveAlerts()

	// 获取异常数据
	var anomalies []Anomaly
	a.anomalies.Range(func(key, value interface{}) bool {
		timestamp := key.(int64)
		if time.Unix(timestamp, 0).After(timeRange.Start) && time.Unix(timestamp, 0).Before(timeRange.End) {
			if anomalyList, ok := value.([]Anomaly); ok {
				anomalies = append(anomalies, anomalyList...)
			}
		}
		return true
	})

	// 生成报告摘要
	summary := a.generateReportSummary(metrics, alerts, anomalies)

	return &MonitorReport{
		Type:        reportType,
		TimeRange:   timeRange,
		Summary:     summary,
		Metrics:     metrics,
		Trends:      trends,
		Alerts:      alerts,
		Anomalies:   anomalies,
		GeneratedAt: now,
		GeneratedBy: "unified_monitor_aggregator",
	}, nil
}

// ExportMetrics 导出指标数据
func (a *MonitorAggregator) ExportMetrics(format ExportFormat, timeRange TimeRange) ([]byte, error) {
	// 获取聚合指标
	metrics, err := a.GetAggregatedMetrics(timeRange)
	if err != nil {
		return nil, fmt.Errorf("获取指标数据失败: %v", err)
	}

	switch format {
	case ExportFormatJSON:
		return a.exportAsJSON(metrics)
	case ExportFormatCSV:
		return a.exportAsCSV(metrics)
	case ExportFormatXML:
		return a.exportAsXML(metrics)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// generateReportSummary 生成报告摘要
func (a *MonitorAggregator) generateReportSummary(metrics *AggregatedMetrics, alerts []Alert, anomalies []Anomaly) *ReportSummary {
	// 计算基本统计
	totalConnections := int64(0)
	totalDevices := int64(0)
	if metrics.ConnectionMetrics != nil {
		totalConnections = metrics.ConnectionMetrics.TotalConnections.Count
	}
	if metrics.DeviceMetrics != nil {
		totalDevices = metrics.DeviceMetrics.TotalDevices.Count
	}

	// 计算平均正常运行时间
	averageUptime := 0.0
	if metrics.DeviceMetrics != nil && metrics.DeviceMetrics.DeviceUptime.Count > 0 {
		averageUptime = metrics.DeviceMetrics.DeviceUptime.Avg
	}

	// 计算错误率
	errorRate := 0.0
	if metrics.PerformanceMetrics != nil {
		errorRate = metrics.PerformanceMetrics.ErrorRate.Avg
	}

	// 计算性能评分
	performanceScore := 100.0 - errorRate
	if performanceScore < 0 {
		performanceScore = 0
	}

	// 健康评分计算已删除，使用简化评分
	overallHealthScore := 100.0 // 简化为固定值

	// 统计关键告警
	criticalAlerts := int64(0)
	for _, alert := range alerts {
		if alert.Severity == AlertSeverityCritical {
			criticalAlerts++
		}
	}

	return &ReportSummary{
		TotalConnections: totalConnections,
		TotalDevices:     totalDevices,
		AverageUptime:    averageUptime,
		ErrorRate:        errorRate,
		PerformanceScore: performanceScore,
		HealthScore:      overallHealthScore,
		CriticalAlerts:   criticalAlerts,
		ResolvedIssues:   int64(len(anomalies)), // 简化实现
	}
}

// === 导出方法实现 ===

// exportAsJSON 导出为JSON格式
func (a *MonitorAggregator) exportAsJSON(metrics *AggregatedMetrics) ([]byte, error) {
	return json.Marshal(metrics)
}

// exportAsCSV 导出为CSV格式
func (a *MonitorAggregator) exportAsCSV(metrics *AggregatedMetrics) ([]byte, error) {
	var buffer bytes.Buffer

	// CSV头部
	buffer.WriteString("Metric,Min,Max,Avg,Sum,Count,P50,P90,P95,P99\n")

	// 连接指标
	if metrics.ConnectionMetrics != nil {
		a.writeMetricToCSV(&buffer, "TotalConnections", metrics.ConnectionMetrics.TotalConnections)
		a.writeMetricToCSV(&buffer, "ActiveConnections", metrics.ConnectionMetrics.ActiveConnections)
		a.writeMetricToCSV(&buffer, "ConnectionDuration", metrics.ConnectionMetrics.ConnectionDuration)
		a.writeMetricToCSV(&buffer, "DataTransferred", metrics.ConnectionMetrics.DataTransferred)
	}

	// 设备指标
	if metrics.DeviceMetrics != nil {
		a.writeMetricToCSV(&buffer, "TotalDevices", metrics.DeviceMetrics.TotalDevices)
		a.writeMetricToCSV(&buffer, "OnlineDevices", metrics.DeviceMetrics.OnlineDevices)
		a.writeMetricToCSV(&buffer, "DeviceUptime", metrics.DeviceMetrics.DeviceUptime)
		a.writeMetricToCSV(&buffer, "HeartbeatFrequency", metrics.DeviceMetrics.HeartbeatFrequency)
	}

	// 性能指标
	if metrics.PerformanceMetrics != nil {
		a.writeMetricToCSV(&buffer, "Latency", metrics.PerformanceMetrics.Latency)
		a.writeMetricToCSV(&buffer, "Throughput", metrics.PerformanceMetrics.Throughput)
		a.writeMetricToCSV(&buffer, "ErrorRate", metrics.PerformanceMetrics.ErrorRate)
		a.writeMetricToCSV(&buffer, "Availability", metrics.PerformanceMetrics.Availability)
	}

	return buffer.Bytes(), nil
}

// exportAsXML 导出为XML格式
func (a *MonitorAggregator) exportAsXML(metrics *AggregatedMetrics) ([]byte, error) {
	var buffer bytes.Buffer

	buffer.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buffer.WriteString("<AggregatedMetrics>\n")

	// 时间范围
	buffer.WriteString("  <TimeRange>\n")
	buffer.WriteString(fmt.Sprintf("    <Start>%s</Start>\n", metrics.TimeRange.Start.Format(time.RFC3339)))
	buffer.WriteString(fmt.Sprintf("    <End>%s</End>\n", metrics.TimeRange.End.Format(time.RFC3339)))
	buffer.WriteString("  </TimeRange>\n")

	// 连接指标
	if metrics.ConnectionMetrics != nil {
		buffer.WriteString("  <ConnectionMetrics>\n")
		a.writeMetricToXML(&buffer, "TotalConnections", metrics.ConnectionMetrics.TotalConnections)
		a.writeMetricToXML(&buffer, "ActiveConnections", metrics.ConnectionMetrics.ActiveConnections)
		buffer.WriteString("  </ConnectionMetrics>\n")
	}

	// 设备指标
	if metrics.DeviceMetrics != nil {
		buffer.WriteString("  <DeviceMetrics>\n")
		a.writeMetricToXML(&buffer, "TotalDevices", metrics.DeviceMetrics.TotalDevices)
		a.writeMetricToXML(&buffer, "OnlineDevices", metrics.DeviceMetrics.OnlineDevices)
		buffer.WriteString("  </DeviceMetrics>\n")
	}

	buffer.WriteString("</AggregatedMetrics>\n")

	return buffer.Bytes(), nil
}

// writeMetricToCSV 将指标写入CSV
func (a *MonitorAggregator) writeMetricToCSV(buffer *bytes.Buffer, name string, summary MetricSummary) {
	buffer.WriteString(fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%d,%.2f,%.2f,%.2f,%.2f\n",
		name, summary.Min, summary.Max, summary.Avg, summary.Sum, summary.Count,
		summary.P50, summary.P90, summary.P95, summary.P99))
}

// writeMetricToXML 将指标写入XML
func (a *MonitorAggregator) writeMetricToXML(buffer *bytes.Buffer, name string, summary MetricSummary) {
	buffer.WriteString(fmt.Sprintf("    <%s>\n", name))
	buffer.WriteString(fmt.Sprintf("      <Min>%.2f</Min>\n", summary.Min))
	buffer.WriteString(fmt.Sprintf("      <Max>%.2f</Max>\n", summary.Max))
	buffer.WriteString(fmt.Sprintf("      <Avg>%.2f</Avg>\n", summary.Avg))
	buffer.WriteString(fmt.Sprintf("      <Sum>%.2f</Sum>\n", summary.Sum))
	buffer.WriteString(fmt.Sprintf("      <Count>%d</Count>\n", summary.Count))
	buffer.WriteString(fmt.Sprintf("    </%s>\n", name))
}

// === 内部辅助方法 ===

// aggregateConnectionMetrics 聚合连接指标
func (a *MonitorAggregator) aggregateConnectionMetrics(connMetrics map[uint64]*ConnectionMetrics) *AggregatedConnectionMetrics {
	if len(connMetrics) == 0 {
		return &AggregatedConnectionMetrics{}
	}

	var totalConns, activeConns, totalBytes []float64
	var durations []float64

	for _, metrics := range connMetrics {
		totalConns = append(totalConns, 1)
		if metrics.Status == "connected" {
			activeConns = append(activeConns, 1)
			if !metrics.ConnectedAt.IsZero() {
				duration := time.Since(metrics.ConnectedAt).Seconds()
				durations = append(durations, duration)
			}
		}
		totalBytes = append(totalBytes, float64(metrics.BytesReceived+metrics.BytesSent))
	}

	return &AggregatedConnectionMetrics{
		TotalConnections:   a.calculateMetricSummary(totalConns),
		ActiveConnections:  a.calculateMetricSummary(activeConns),
		ConnectionDuration: a.calculateMetricSummary(durations),
		DataTransferred:    a.calculateMetricSummary(totalBytes),
	}
}

// aggregateDeviceMetrics 聚合设备指标
func (a *MonitorAggregator) aggregateDeviceMetrics(deviceMetrics map[string]*DeviceMetrics) *AggregatedDeviceMetrics {
	if len(deviceMetrics) == 0 {
		return &AggregatedDeviceMetrics{}
	}

	var totalDevices, onlineDevices, heartbeats []float64
	var uptimes []float64

	for _, metrics := range deviceMetrics {
		totalDevices = append(totalDevices, 1)
		if metrics.Status == "online" {
			onlineDevices = append(onlineDevices, 1)
			if !metrics.ConnectedAt.IsZero() {
				uptime := time.Since(metrics.ConnectedAt).Seconds()
				uptimes = append(uptimes, uptime)
			}
		}
		heartbeats = append(heartbeats, float64(metrics.HeartbeatCount))
	}

	return &AggregatedDeviceMetrics{
		TotalDevices:       a.calculateMetricSummary(totalDevices),
		OnlineDevices:      a.calculateMetricSummary(onlineDevices),
		DeviceUptime:       a.calculateMetricSummary(uptimes),
		HeartbeatFrequency: a.calculateMetricSummary(heartbeats),
	}
}

// aggregateSystemMetrics 聚合系统指标
func (a *MonitorAggregator) aggregateSystemMetrics(systemMetrics *SystemMetrics) *AggregatedSystemMetrics {
	if systemMetrics == nil {
		return &AggregatedSystemMetrics{}
	}

	return &AggregatedSystemMetrics{
		CPUUsage:       a.calculateMetricSummary([]float64{systemMetrics.CPUUsage}),
		MemoryUsage:    a.calculateMetricSummary([]float64{systemMetrics.MemoryUsage}),
		GoroutineCount: a.calculateMetricSummary([]float64{float64(systemMetrics.GoroutineCount)}),
		GCPerformance:  a.calculateMetricSummary([]float64{float64(systemMetrics.GCPauseTotal)}),
	}
}

// aggregatePerformanceMetrics 聚合性能指标
func (a *MonitorAggregator) aggregatePerformanceMetrics(perfStats *PerformanceStats) *AggregatedPerformanceMetrics {
	if perfStats == nil {
		return &AggregatedPerformanceMetrics{}
	}

	latencyMs := float64(perfStats.AverageLatency.Nanoseconds()) / 1e6

	return &AggregatedPerformanceMetrics{
		Latency:      a.calculateMetricSummary([]float64{latencyMs}),
		Throughput:   a.calculateMetricSummary([]float64{perfStats.Throughput}),
		ErrorRate:    a.calculateMetricSummary([]float64{perfStats.ErrorRate}),
		Availability: a.calculateMetricSummary([]float64{100.0 - perfStats.ErrorRate}),
	}
}

// aggregateCustomMetrics 聚合自定义指标
func (a *MonitorAggregator) aggregateCustomMetrics(customMetrics map[string]interface{}) map[string]*AggregatedCustomMetric {
	result := make(map[string]*AggregatedCustomMetric)

	for name, metricInterface := range customMetrics {
		if metricData, ok := metricInterface.(map[string]interface{}); ok {
			if value, ok := metricData["value"].(float64); ok {
				tags := make(map[string]string)
				if tagsInterface, exists := metricData["tags"]; exists {
					if tagsMap, ok := tagsInterface.(map[string]string); ok {
						tags = tagsMap
					}
				}

				result[name] = &AggregatedCustomMetric{
					Name:    name,
					Summary: a.calculateMetricSummary([]float64{value}),
					Tags:    tags,
				}
			}
		}
	}

	return result
}

// calculateMetricSummary 计算指标摘要
func (a *MonitorAggregator) calculateMetricSummary(values []float64) MetricSummary {
	if len(values) == 0 {
		return MetricSummary{}
	}

	// 排序用于计算百分位数
	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)

	// 简单排序
	for i := 0; i < len(sortedValues); i++ {
		for j := i + 1; j < len(sortedValues); j++ {
			if sortedValues[i] > sortedValues[j] {
				sortedValues[i], sortedValues[j] = sortedValues[j], sortedValues[i]
			}
		}
	}

	// 计算基本统计
	minVal := sortedValues[0]
	maxVal := sortedValues[len(sortedValues)-1]
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))

	// 计算百分位数
	p50 := a.percentile(sortedValues, 50)
	p90 := a.percentile(sortedValues, 90)
	p95 := a.percentile(sortedValues, 95)
	p99 := a.percentile(sortedValues, 99)

	// 计算标准差
	variance := 0.0
	for _, v := range values {
		variance += (v - avg) * (v - avg)
	}
	stdDev := 0.0
	if len(values) > 1 {
		stdDev = variance / float64(len(values)-1)
	}

	return MetricSummary{
		Min:    minVal,
		Max:    maxVal,
		Avg:    avg,
		Sum:    sum,
		Count:  int64(len(values)),
		P50:    p50,
		P90:    p90,
		P95:    p95,
		P99:    p99,
		StdDev: stdDev,
	}
}

// percentile 计算百分位数
func (a *MonitorAggregator) percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if len(sortedValues) == 1 {
		return sortedValues[0]
	}

	index := (p / 100.0) * float64(len(sortedValues)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sortedValues) {
		return sortedValues[len(sortedValues)-1]
	}

	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}

// calculateTrend 计算趋势方向和变化率
func (a *MonitorAggregator) calculateTrend(dataPoints []TrendDataPoint) (TrendDirection, float64) {
	if len(dataPoints) < 2 {
		return TrendFlat, 0
	}

	first := dataPoints[0].Value
	last := dataPoints[len(dataPoints)-1].Value

	changeRate := ((last - first) / first) * 100

	if changeRate > 5 {
		return TrendUp, changeRate
	} else if changeRate < -5 {
		return TrendDown, changeRate
	} else {
		return TrendFlat, changeRate
	}
}

// analyzeOverallHealth 分析总体健康状态
func (a *MonitorAggregator) analyzeOverallHealth(connStats *ConnectionStats, deviceStats *DeviceStats, perfStats *PerformanceStats, alertStats *AlertStats) HealthLevel {
	// 计算健康评分
	score := 100.0

	// 连接健康度 (25%)
	if connStats.TotalConnections > 0 {
		activeRatio := float64(connStats.ActiveConnections) / float64(connStats.TotalConnections)
		score -= (1 - activeRatio) * 25
	}

	// 设备健康度 (25%)
	if deviceStats.TotalDevices > 0 {
		onlineRatio := float64(deviceStats.OnlineDevices) / float64(deviceStats.TotalDevices)
		score -= (1 - onlineRatio) * 25
	}

	// 性能健康度 (25%)
	score -= perfStats.ErrorRate * 0.25

	// 告警健康度 (25%)
	if alertStats.TotalAlerts > 0 {
		activeAlertRatio := float64(alertStats.ActiveAlerts) / float64(alertStats.TotalAlerts)
		score -= activeAlertRatio * 25
	}

	// 确定健康等级
	switch {
	case score >= 90:
		return HealthExcellent
	case score >= 75:
		return HealthGood
	case score >= 60:
		return HealthWarning
	default:
		return HealthCritical
	}
}

// detectCriticalIssues 检测关键问题
func (a *MonitorAggregator) detectCriticalIssues(connStats *ConnectionStats, deviceStats *DeviceStats, perfStats *PerformanceStats, alertStats *AlertStats) []CriticalIssue {
	var issues []CriticalIssue
	now := time.Now()

	// 检测连接问题
	if connStats.ActiveConnections == 0 && connStats.TotalConnections > 0 {
		issues = append(issues, CriticalIssue{
			Type:               "connection",
			Description:        "所有连接都不活跃",
			Severity:           "critical",
			DetectedAt:         now,
			AffectedComponents: []string{"network", "connections"},
		})
	}

	// 检测设备问题
	if deviceStats.OnlineDevices == 0 && deviceStats.TotalDevices > 0 {
		issues = append(issues, CriticalIssue{
			Type:               "device",
			Description:        "所有设备都离线",
			Severity:           "critical",
			DetectedAt:         now,
			AffectedComponents: []string{"devices", "network"},
		})
	}

	// 检测性能问题
	if perfStats.ErrorRate > 10 {
		issues = append(issues, CriticalIssue{
			Type:               "performance",
			Description:        fmt.Sprintf("错误率过高: %.2f%%", perfStats.ErrorRate),
			Severity:           "high",
			DetectedAt:         now,
			AffectedComponents: []string{"performance", "system"},
		})
	}

	// 检测告警问题
	if alertStats.CriticalAlerts > 0 {
		issues = append(issues, CriticalIssue{
			Type:               "alerts",
			Description:        fmt.Sprintf("存在 %d 个关键告警", alertStats.CriticalAlerts),
			Severity:           "critical",
			DetectedAt:         now,
			AffectedComponents: []string{"monitoring", "alerts"},
		})
	}

	return issues
}

// generateRecommendations 生成建议
func (a *MonitorAggregator) generateRecommendations(issues []CriticalIssue, perfStats *PerformanceStats) []Recommendation {
	var recommendations []Recommendation

	for _, issue := range issues {
		switch issue.Type {
		case "connection":
			recommendations = append(recommendations, Recommendation{
				Type:        "network",
				Description: "检查网络连接和服务器状态",
				Priority:    "high",
				Action:      "restart_network_service",
			})
		case "device":
			recommendations = append(recommendations, Recommendation{
				Type:        "device",
				Description: "检查设备网络连接和电源状态",
				Priority:    "high",
				Action:      "check_device_status",
			})
		case "performance":
			recommendations = append(recommendations, Recommendation{
				Type:        "performance",
				Description: "优化系统性能，检查资源使用情况",
				Priority:    "medium",
				Action:      "optimize_performance",
			})
		case "alerts":
			recommendations = append(recommendations, Recommendation{
				Type:        "monitoring",
				Description: "处理关键告警，检查系统状态",
				Priority:    "critical",
				Action:      "handle_alerts",
			})
		}
	}

	// 基于性能统计生成额外建议
	if perfStats.AverageLatency > 1*time.Second {
		recommendations = append(recommendations, Recommendation{
			Type:        "performance",
			Description: "平均延迟过高，建议优化网络或增加服务器资源",
			Priority:    "medium",
			Action:      "optimize_latency",
		})
	}

	return recommendations
}

// abs 计算绝对值
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// === 后台协程实现 ===

// aggregationRoutine 聚合协程
func (a *MonitorAggregator) aggregationRoutine() {
	ticker := time.NewTicker(a.aggregationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := a.AggregateMetrics(a.aggregationInterval); err != nil {
				logger.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Error("监控数据聚合失败")
			}

		case <-a.stopChan:
			return
		}
	}
}

// 异常检测协程已删除，简化监控架构

// cleanupRoutine 清理协程
func (a *MonitorAggregator) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.performCleanup()

		case <-a.stopChan:
			return
		}
	}
}

// performCleanup 执行清理操作
func (a *MonitorAggregator) performCleanup() {
	now := time.Now()
	cutoffTime := now.Add(-a.retentionPeriod)

	// 清理过期的聚合数据
	cleanupCount := 0
	a.aggregatedData.Range(func(key, value interface{}) bool {
		timestamp := key.(int64)
		if time.Unix(timestamp, 0).Before(cutoffTime) {
			a.aggregatedData.Delete(key)
			cleanupCount++
		}
		return true
	})

	// 清理过期的异常数据
	a.anomalies.Range(func(key, value interface{}) bool {
		timestamp := key.(int64)
		if time.Unix(timestamp, 0).Before(cutoffTime) {
			a.anomalies.Delete(key)
		}
		return true
	})

	// 清理过期的趋势数据
	a.trendData.Range(func(key, value interface{}) bool {
		metric := key.(string)
		if dataPoints, ok := value.([]TrendDataPoint); ok {
			var filteredPoints []TrendDataPoint
			for _, point := range dataPoints {
				if point.Timestamp.After(cutoffTime) {
					filteredPoints = append(filteredPoints, point)
				}
			}
			if len(filteredPoints) != len(dataPoints) {
				if len(filteredPoints) == 0 {
					a.trendData.Delete(metric)
				} else {
					a.trendData.Store(metric, filteredPoints)
				}
			}
		}
		return true
	})

	if cleanupCount > 0 {
		logger.WithFields(logrus.Fields{
			"cleanup_count": cleanupCount,
			"cutoff_time":   cutoffTime,
		}).Info("监控数据清理完成")
	}
}

// === 全局实例管理 ===

var (
	globalMonitorAggregator     *MonitorAggregator
	globalMonitorAggregatorOnce sync.Once
)

// GetGlobalMonitorAggregator 获取全局监控数据聚合器实例
func GetGlobalMonitorAggregator() *MonitorAggregator {
	globalMonitorAggregatorOnce.Do(func() {
		globalMonitorAggregator = NewMonitorAggregator(GetGlobalUnifiedMonitor())
		if err := globalMonitorAggregator.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局监控数据聚合器失败")
		}
	})
	return globalMonitorAggregator
}

// SetGlobalMonitorAggregator 设置全局监控数据聚合器实例（用于测试）
func SetGlobalMonitorAggregator(aggregator *MonitorAggregator) {
	globalMonitorAggregator = aggregator
}

// === 接口实现检查 ===

// 确保MonitorAggregator实现了IMonitorAggregator接口
var _ IMonitorAggregator = (*MonitorAggregator)(nil)
