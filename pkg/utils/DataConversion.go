package utils

import (
	"encoding/hex"
	"fmt"
)

// phyIDToDecimal 将设备物理ID（如 "04A26CF3"）转为二维码下方十进制编号
func phyIDToDecimal(phyidHex string) (uint32, error) {
	// 1. 物理ID 4字节16进制字符串（如"04A26CF3"）
	if len(phyidHex) != 8 {
		return 0, fmt.Errorf("phyid hex length must be 8")
	}
	// 2. 转为字节序列
	bs, err := hex.DecodeString(phyidHex)
	if err != nil {
		return 0, err
	}
	// 3. 取后三字节，拼成大端整数
	id := uint32(bs[1])<<16 | uint32(bs[2])<<8 | uint32(bs[3])
	return id, nil
}
