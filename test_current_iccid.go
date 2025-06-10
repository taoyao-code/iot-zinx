package main

import (
	"encoding/hex"
	"fmt"
)

func main() {
	// 从日志中看到的ICCID十六进制数据
	hexStr := "3839383630344439313632333930343838323937"
	
	fmt.Printf("原始十六进制字符串: %s\n", hexStr)
	fmt.Printf("长度: %d\n", len(hexStr))
	
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Printf("解码失败: %v\n", err)
		return
	}
	
	decodedStr := string(decoded)
	fmt.Printf("解码后: %s\n", decodedStr)
	fmt.Printf("解码后长度: %d\n", len(decodedStr))
	
	// 验证是否为有效的ICCID
	isValidICCID := len(decodedStr) >= 19 && len(decodedStr) <= 25
	fmt.Printf("是否为有效ICCID长度: %v\n", isValidICCID)
	
	// 检查是否全是数字
	allDigits := true
	for _, r := range decodedStr {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	fmt.Printf("是否全为数字: %v\n", allDigits)
}