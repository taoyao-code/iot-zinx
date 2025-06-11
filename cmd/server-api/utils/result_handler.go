package utils

import (
	"encoding/json"
	"fmt"
	"log"
)

// HandleOperationResult 处理操作结果
func HandleOperationResult(result interface{}, err error) {
	if err != nil {
		fmt.Printf("操作失败: %s\n", err)
		return
	}

	// 格式化输出JSON结果
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("JSON格式化失败: %s\n", err)
		fmt.Printf("原始结果: %+v\n", result)
		return
	}
	fmt.Println("操作成功，结果:")
	fmt.Println(string(jsonData))
}
