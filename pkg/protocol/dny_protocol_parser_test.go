package protocol

import (
	"testing"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

func TestParseDNYProtocolData(t *testing.T) {
	// 测试ICCID消息解析
	t.Run("ICCID Message", func(t *testing.T) {
		iccidData := []byte("ICCID12345678901234567890")
		msg, err := ParseDNYProtocolData(iccidData)
		
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		
		if msg.MessageType != "iccid" {
			t.Errorf("Expected MessageType 'iccid', got: %s", msg.MessageType)
		}
		
		if msg.ICCIDValue != "12345678901234567890" {
			t.Errorf("Expected ICCID '12345678901234567890', got: %s", msg.ICCIDValue)
		}
	})

	// 测试Link心跳消息解析
	t.Run("Link Heartbeat", func(t *testing.T) {
		linkData := []byte("link")
		msg, err := ParseDNYProtocolData(linkData)
		
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		
		if msg.MessageType != "heartbeat_link" {
			t.Errorf("Expected MessageType 'heartbeat_link', got: %s", msg.MessageType)
		}
	})

	// 测试空数据错误处理
	t.Run("Empty Data", func(t *testing.T) {
		emptyData := []byte{}
		msg, err := ParseDNYProtocolData(emptyData)
		
		if err == nil {
			t.Fatal("Expected error for empty data, got none")
		}
		
		if msg.MessageType != "error" {
			t.Errorf("Expected MessageType 'error', got: %s", msg.MessageType)
		}
	})

	// 测试IsSpecialMessage函数
	t.Run("IsSpecialMessage", func(t *testing.T) {
		// 测试ICCID
		iccidData := []byte("12345678901234567890") // 20位数字
		if !IsSpecialMessage(iccidData) {
			t.Error("Expected ICCID to be special message")
		}

		// 测试link心跳
		linkData := []byte("link")
		if !IsSpecialMessage(linkData) {
			t.Error("Expected link to be special message")
		}

		// 测试普通数据
		normalData := []byte("DNYabc123")
		if IsSpecialMessage(normalData) {
			t.Error("Expected normal data not to be special message")
		}
	})
}

func TestConstants(t *testing.T) {
	// 验证常量是否正确定义
	if constants.IOT_SIM_CARD_LENGTH != 20 {
		t.Errorf("Expected IOT_SIM_CARD_LENGTH to be 20, got: %d", constants.IOT_SIM_CARD_LENGTH)
	}

	if constants.IOT_LINK_HEARTBEAT != "link" {
		t.Errorf("Expected IOT_LINK_HEARTBEAT to be 'link', got: %s", constants.IOT_LINK_HEARTBEAT)
	}

	if constants.DNY_MIN_PACKET_LEN != 12 {
		t.Errorf("Expected DNY_MIN_PACKET_LEN to be 12, got: %d", constants.DNY_MIN_PACKET_LEN)
	}
}
