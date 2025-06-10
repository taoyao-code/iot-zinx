package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// IsAllDigits 检查是否为合法的ICCID格式（数字和十六进制字符A-F）
func IsAllDigits(data []byte) bool {
	return strings.IndexFunc(string(data), func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') || (r >= 'a' && r <= 'f'))
	}) == -1
}

// extractICCID 模拟实际的函数
func extractICCID(data []byte) (string, bool) {
	dataStr := string(data)

	// 排除DNY协议包：检查是否以"DNY"开头
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		fmt.Printf("排除DNY协议包\n")
		return "", false
	}

	// 尝试作为十六进制字符串解码（如：3839383630344439313632333930343838323937）
	if len(dataStr)%2 == 0 && len(dataStr) >= 38 && len(dataStr) <= 50 {
		fmt.Printf("尝试十六进制解码，长度: %d\n", len(dataStr))
		if decoded, err := hex.DecodeString(dataStr); err == nil {
			decodedStr := string(decoded)
			fmt.Printf("解码成功: %s\n", decodedStr)
			// 验证解码后的字符串是否为有效ICCID（19-25位，支持十六进制字符）
			if len(decodedStr) >= 19 && len(decodedStr) <= 25 && IsAllDigits([]byte(decodedStr)) {
				fmt.Printf("十六进制解码验证通过\n")
				return decodedStr, true
			} else {
				fmt.Printf("十六进制解码验证失败: 长度=%d, IsAllDigits=%v\n", len(decodedStr), IsAllDigits([]byte(decodedStr)))
			}
		} else {
			fmt.Printf("十六进制解码失败: %v\n", err)
		}
	}

	// 直接检查是否为ICCID格式（19-25位，支持十六进制字符A-F）
	if len(dataStr) >= 19 && len(dataStr) <= 25 && IsAllDigits([]byte(dataStr)) {
		fmt.Printf("直接ICCID格式验证通过\n")
		return dataStr, true
	} else {
		fmt.Printf("直接ICCID格式验证失败: 长度=%d, IsAllDigits=%v\n", len(dataStr), IsAllDigits([]byte(dataStr)))
	}

	return "", false
}

func main() {
	// 从日志中看到的ICCID十六进制数据
	hexStr := "3839383630344439313632333930343838323937"
	fmt.Printf("测试数据: %s\n", hexStr)
	fmt.Printf("数据长度: %d\n", len(hexStr))

	data := []byte(hexStr)
	iccid, ok := extractICCID(data)
	fmt.Printf("提取结果: iccid=%s, ok=%v\n", iccid, ok)
}
