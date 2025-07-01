package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
)

var (
	configFile = flag.String("config", "configs/gateway.yaml", "配置文件路径")
	format     = flag.String("format", "table", "输出格式: table, json")
	watch      = flag.Bool("watch", false, "持续监控模式")
	interval   = flag.Duration("interval", 5*time.Second, "监控间隔")
)

func main() {
	flag.Parse()

	// 加载配置
	if err := config.LoadConfig(*configFile); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化通知系统
	ctx := context.Background()
	if err := notification.InitGlobalNotificationIntegrator(ctx); err != nil {
		log.Fatalf("初始化通知系统失败: %v", err)
	}
	defer notification.StopGlobalNotificationIntegrator(ctx)

	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		fmt.Println("⚠️  通知系统未启用")
		return
	}

	if *watch {
		// 持续监控模式
		fmt.Printf("🔄 开始监控通知推送统计 (间隔: %v)\n", *interval)
		fmt.Println("按 Ctrl+C 退出")
		fmt.Println()

		for {
			printStats(integrator, *format)
			time.Sleep(*interval)
			// 清屏
			fmt.Print("\033[2J\033[H")
		}
	} else {
		// 单次查询模式
		printStats(integrator, *format)
	}
}

func printStats(integrator *notification.NotificationIntegrator, format string) {
	stats := integrator.GetDetailedStats()
	if stats == nil {
		fmt.Println("无法获取统计信息")
		return
	}

	switch format {
	case "json":
		printJSONStats(stats)
	case "table":
		printTableStats(stats)
	default:
		fmt.Printf("不支持的格式: %s\n", format)
	}
}

func printJSONStats(stats *notification.NotificationStats) {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		fmt.Printf("序列化统计信息失败: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printTableStats(stats *notification.NotificationStats) {
	fmt.Printf("📊 通知推送统计报告 - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 80))

	// 全局统计
	fmt.Printf("📈 全局统计:\n")
	fmt.Printf("  总发送数:     %d\n", stats.TotalSent)
	fmt.Printf("  成功数:       %d\n", stats.TotalSuccess)
	fmt.Printf("  失败数:       %d\n", stats.TotalFailed)
	fmt.Printf("  成功率:       %.2f%%\n", stats.SuccessRate)
	fmt.Printf("  平均响应时间: %s\n", stats.AvgResponseTime.String())
	fmt.Printf("  最后更新:     %s\n", stats.LastUpdateTime.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 端点统计
	if len(stats.EndpointStats) > 0 {
		fmt.Printf("🎯 端点统计:\n")
		fmt.Printf("%-20s %-8s %-8s %-8s %-10s %-15s %-19s %-19s\n",
			"端点名称", "发送数", "成功数", "失败数", "成功率", "平均响应时间", "最后成功", "最后失败")
		fmt.Println(strings.Repeat("-", 120))

		for _, endpointStats := range stats.EndpointStats {
			lastSuccess := "-"
			if !endpointStats.LastSuccess.IsZero() {
				lastSuccess = endpointStats.LastSuccess.Format("01-02 15:04:05")
			}

			lastFailure := "-"
			if !endpointStats.LastFailure.IsZero() {
				lastFailure = endpointStats.LastFailure.Format("01-02 15:04:05")
			}

			fmt.Printf("%-20s %-8d %-8d %-8d %-9.2f%% %-15s %-19s %-19s\n",
				endpointStats.Name,
				endpointStats.TotalSent,
				endpointStats.TotalSuccess,
				endpointStats.TotalFailed,
				endpointStats.SuccessRate,
				endpointStats.AvgResponseTime.String(),
				lastSuccess,
				lastFailure)
		}
	}

	fmt.Println()
}
