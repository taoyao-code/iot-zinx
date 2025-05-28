package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// 服务器地址配置
const (
	HTTPServerAddr = "http://localhost:8080"
	TCPServerAddr  = "localhost:7054"
)

// API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// 设备信息结构
type DeviceInfo struct {
	DeviceID          string  `json:"deviceId"`
	ICCID             string  `json:"iccid"`
	IsOnline          bool    `json:"isOnline"`
	Status            string  `json:"status"`
	LastHeartbeat     int64   `json:"lastHeartbeat"`
	LastHeartbeatTime string  `json:"lastHeartbeatTime"`
	TimeSinceHeart    float64 `json:"timeSinceHeart"`
	RemoteAddr        string  `json:"remoteAddr"`
	ConnID            uint32  `json:"connId"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("      充电设备网关测试工具")
	fmt.Println("========================================")
	fmt.Printf("HTTP服务器地址: %s\n", HTTPServerAddr)
	fmt.Printf("TCP服务器地址: %s\n", TCPServerAddr)
	fmt.Println("========================================")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		printMenu()
		fmt.Print("请选择操作 (输入数字): ")

		if !scanner.Scan() {
			break
		}

		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			listDevices()
		case "2":
			queryDeviceStatus(scanner)
		case "3":
			sendHeartbeat(scanner)
		case "4":
			startCharging(scanner)
		case "5":
			stopCharging(scanner)
		case "6":
			sendCustomCommand(scanner)
		case "7":
			showServerHealth()
		case "8":
			monitorDevices()
		case "0", "q", "quit", "exit":
			fmt.Println("退出测试工具...")
			return
		default:
			fmt.Println("无效选择，请重新输入")
		}

		fmt.Println("\n按回车键继续...")
		scanner.Scan()
	}
}

// printMenu 打印菜单
func printMenu() {
	fmt.Println("\n========== 功能菜单 ==========")
	fmt.Println("1. 查看在线设备列表")
	fmt.Println("2. 查询设备状态")
	fmt.Println("3. 发送心跳测试")
	fmt.Println("4. 开始充电")
	fmt.Println("5. 停止充电")
	fmt.Println("6. 发送自定义命令")
	fmt.Println("7. 服务器健康检查")
	fmt.Println("8. 实时监控设备")
	fmt.Println("0. 退出")
	fmt.Println("=============================")
}

// listDevices 列出所有在线设备
func listDevices() {
	fmt.Println("\n正在获取设备列表...")

	resp, err := http.Get(HTTPServerAddr + "/api/devices")
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	if apiResp.Code != 0 {
		fmt.Printf("API错误: %s\n", apiResp.Message)
		return
	}

	data := apiResp.Data.(map[string]interface{})
	devices := data["devices"].([]interface{})
	total := int(data["total"].(float64))

	fmt.Printf("\n在线设备总数: %d\n", total)
	if total == 0 {
		fmt.Println("暂无在线设备")
		return
	}

	fmt.Println("\n设备列表:")
	fmt.Println("┌─────────────┬──────────────────────┬─────────┬─────────────────────┬─────────────────────┐")
	fmt.Println("│   设备ID    │        ICCID         │  状态   │      远程地址       │    最后心跳时间     │")
	fmt.Println("├─────────────┼──────────────────────┼─────────┼─────────────────────┼─────────────────────┤")

	for _, device := range devices {
		dev := device.(map[string]interface{})
		deviceID := getString(dev, "deviceId")
		iccid := getString(dev, "iccid")
		isOnline := getBool(dev, "isOnline")
		remoteAddr := getString(dev, "remoteAddr")
		heartbeatTime := getString(dev, "lastHeartbeatTime")

		status := "离线"
		if isOnline {
			status = "在线"
		}

		if heartbeatTime == "" {
			heartbeatTime = "无记录"
		}

		// 截断长字符串以适应表格
		if len(deviceID) > 11 {
			deviceID = deviceID[:8] + "..."
		}
		if len(iccid) > 20 {
			iccid = iccid[:17] + "..."
		}
		if len(remoteAddr) > 19 {
			remoteAddr = remoteAddr[:16] + "..."
		}
		if len(heartbeatTime) > 19 {
			heartbeatTime = heartbeatTime[:16] + "..."
		}

		fmt.Printf("│ %-11s │ %-20s │ %-7s │ %-19s │ %-19s │\n",
			deviceID, iccid, status, remoteAddr, heartbeatTime)
	}

	fmt.Println("└─────────────┴──────────────────────┴─────────┴─────────────────────┴─────────────────────┘")
}

// queryDeviceStatus 查询设备详细状态
func queryDeviceStatus(scanner *bufio.Scanner) {
	fmt.Print("\n请输入设备ID: ")
	if !scanner.Scan() {
		return
	}
	deviceID := strings.TrimSpace(scanner.Text())

	if deviceID == "" {
		fmt.Println("设备ID不能为空")
		return
	}

	fmt.Printf("正在查询设备 %s 的状态...\n", deviceID)

	resp, err := http.Get(HTTPServerAddr + "/api/device/" + deviceID + "/status")
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	if apiResp.Code != 0 {
		fmt.Printf("查询失败: %s\n", apiResp.Message)
		return
	}

	data := apiResp.Data.(map[string]interface{})

	fmt.Println("\n设备详细状态:")
	fmt.Println("==========================================")
	fmt.Printf("设备ID: %s\n", getString(data, "deviceId"))
	fmt.Printf("ICCID: %s\n", getString(data, "iccid"))
	fmt.Printf("在线状态: %v\n", getBool(data, "isOnline"))
	fmt.Printf("连接状态: %s\n", getString(data, "status"))
	fmt.Printf("远程地址: %s\n", getString(data, "remoteAddr"))
	fmt.Printf("最后心跳: %s\n", getString(data, "heartbeatTime"))

	if timeSince := getFloat64(data, "timeSinceHeart"); timeSince > 0 {
		fmt.Printf("距上次心跳: %.1f秒\n", timeSince)
	}
	fmt.Println("==========================================")
}

// sendHeartbeat 发送心跳测试
func sendHeartbeat(scanner *bufio.Scanner) {
	fmt.Print("\n请输入设备ID: ")
	if !scanner.Scan() {
		return
	}
	deviceID := strings.TrimSpace(scanner.Text())

	if deviceID == "" {
		fmt.Println("设备ID不能为空")
		return
	}

	// 发送心跳查询命令
	reqData := map[string]interface{}{
		"deviceId":  deviceID,
		"command":   0x81, // 查询设备状态命令
		"data":      "",
		"messageId": uint16(time.Now().Unix() & 0xFFFF),
	}

	fmt.Printf("正在向设备 %s 发送心跳查询...\n", deviceID)

	if sendCommand(reqData) {
		fmt.Println("心跳查询发送成功！")
	}
}

// startCharging 开始充电
func startCharging(scanner *bufio.Scanner) {
	fmt.Print("\n请输入设备ID: ")
	if !scanner.Scan() {
		return
	}
	deviceID := strings.TrimSpace(scanner.Text())

	fmt.Print("请输入端口号 (1-8): ")
	if !scanner.Scan() {
		return
	}
	portStr := strings.TrimSpace(scanner.Text())
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 8 {
		fmt.Println("端口号必须是1-8之间的数字")
		return
	}

	fmt.Print("请选择充电模式 (0=按时间, 1=按电量): ")
	if !scanner.Scan() {
		return
	}
	modeStr := strings.TrimSpace(scanner.Text())
	mode, err := strconv.Atoi(modeStr)
	if err != nil || (mode != 0 && mode != 1) {
		fmt.Println("充电模式必须是0或1")
		return
	}

	var prompt string
	if mode == 0 {
		prompt = "请输入充电时间 (分钟): "
	} else {
		prompt = "请输入充电电量 (0.1度为单位): "
	}

	fmt.Print(prompt)
	if !scanner.Scan() {
		return
	}
	valueStr := strings.TrimSpace(scanner.Text())
	value, err := strconv.Atoi(valueStr)
	if err != nil || value <= 0 {
		fmt.Println("数值必须是正整数")
		return
	}

	fmt.Print("请输入订单号: ")
	if !scanner.Scan() {
		return
	}
	orderNo := strings.TrimSpace(scanner.Text())
	if orderNo == "" {
		orderNo = fmt.Sprintf("TEST%d", time.Now().Unix())
	}

	reqData := map[string]interface{}{
		"deviceId": deviceID,
		"port":     port,
		"mode":     mode,
		"value":    value,
		"orderNo":  orderNo,
	}

	fmt.Printf("正在向设备 %s 端口 %d 发送开始充电命令...\n", deviceID, port)

	body, _ := json.Marshal(reqData)
	resp, err := http.Post(HTTPServerAddr+"/api/charging/start", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	if apiResp.Code == 0 {
		fmt.Println("开始充电命令发送成功！")
	} else {
		fmt.Printf("发送失败: %s\n", apiResp.Message)
	}
}

// stopCharging 停止充电
func stopCharging(scanner *bufio.Scanner) {
	fmt.Print("\n请输入设备ID: ")
	if !scanner.Scan() {
		return
	}
	deviceID := strings.TrimSpace(scanner.Text())

	fmt.Print("请输入端口号 (1-8, 回车表示停止所有端口): ")
	if !scanner.Scan() {
		return
	}
	portStr := strings.TrimSpace(scanner.Text())

	port := 255 // 默认停止所有端口
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 8 {
			fmt.Println("端口号必须是1-8之间的数字")
			return
		}
		port = p
	}

	fmt.Print("请输入订单号 (可选): ")
	if !scanner.Scan() {
		return
	}
	orderNo := strings.TrimSpace(scanner.Text())

	reqData := map[string]interface{}{
		"deviceId": deviceID,
		"port":     port,
		"orderNo":  orderNo,
	}

	if port == 255 {
		fmt.Printf("正在向设备 %s 发送停止所有端口充电命令...\n", deviceID)
	} else {
		fmt.Printf("正在向设备 %s 端口 %d 发送停止充电命令...\n", deviceID, port)
	}

	body, _ := json.Marshal(reqData)
	resp, err := http.Post(HTTPServerAddr+"/api/charging/stop", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	if apiResp.Code == 0 {
		fmt.Println("停止充电命令发送成功！")
	} else {
		fmt.Printf("发送失败: %s\n", apiResp.Message)
	}
}

// sendCustomCommand 发送自定义命令
func sendCustomCommand(scanner *bufio.Scanner) {
	fmt.Print("\n请输入设备ID: ")
	if !scanner.Scan() {
		return
	}
	deviceID := strings.TrimSpace(scanner.Text())

	fmt.Print("请输入命令代码 (16进制，如: 81): ")
	if !scanner.Scan() {
		return
	}
	cmdStr := strings.TrimSpace(scanner.Text())
	cmd, err := strconv.ParseUint(cmdStr, 16, 8)
	if err != nil {
		fmt.Println("命令代码格式错误")
		return
	}

	fmt.Print("请输入数据 (16进制字符串，可选): ")
	if !scanner.Scan() {
		return
	}
	dataHex := strings.TrimSpace(scanner.Text())

	// 验证hex数据
	if dataHex != "" {
		if _, err := hex.DecodeString(dataHex); err != nil {
			fmt.Println("数据格式错误，必须是有效的16进制字符串")
			return
		}
	}

	reqData := map[string]interface{}{
		"deviceId":  deviceID,
		"command":   cmd,
		"data":      dataHex,
		"messageId": uint16(time.Now().Unix() & 0xFFFF),
	}

	fmt.Printf("正在向设备 %s 发送自定义命令 0x%02X...\n", deviceID, cmd)

	if sendCommand(reqData) {
		fmt.Println("自定义命令发送成功！")
	}
}

// showServerHealth 显示服务器健康状态
func showServerHealth() {
	fmt.Println("\n正在检查服务器健康状态...")

	resp, err := http.Get(HTTPServerAddr + "/health")
	if err != nil {
		fmt.Printf("健康检查失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	fmt.Println("\n服务器健康状态:")
	fmt.Println("==========================================")
	if apiResp.Code == 0 {
		fmt.Printf("状态: ✅ %s\n", apiResp.Message)
		fmt.Printf("HTTP服务: 正常运行在 %s\n", HTTPServerAddr)
		fmt.Printf("TCP服务: 预期运行在 %s\n", TCPServerAddr)
	} else {
		fmt.Printf("状态: ❌ %s\n", apiResp.Message)
	}
	fmt.Printf("检查时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("==========================================")
}

// monitorDevices 实时监控设备
func monitorDevices() {
	fmt.Println("\n开始实时监控设备状态...")
	fmt.Println("按 Ctrl+C 停止监控\n")

	for i := 0; i < 10; i++ { // 监控10次
		fmt.Printf("\n========== 第 %d 次刷新 ==========\n", i+1)
		listDevices()

		if i < 9 {
			fmt.Println("\n3秒后刷新...")
			time.Sleep(3 * time.Second)
		}
	}

	fmt.Println("\n监控结束")
}

// sendCommand 发送DNY命令的通用方法
func sendCommand(reqData map[string]interface{}) bool {
	body, _ := json.Marshal(reqData)
	resp, err := http.Post(HTTPServerAddr+"/api/command/dny", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return false
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return false
	}

	if apiResp.Code != 0 {
		fmt.Printf("发送失败: %s\n", apiResp.Message)
		return false
	}

	// 显示发送的数据包信息
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if packetHex, exists := data["packetHex"]; exists {
			fmt.Printf("发送的数据包: %s\n", packetHex)
		}
	}

	return true
}

// 辅助函数
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok && val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getFloat64(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok && val != nil {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}
