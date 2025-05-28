package unit

import (
	"testing"

	"github.com/bujia-iot/iot-zinx/internal/app/service"
)

func TestDeviceService_ValidateCard(t *testing.T) {
	// 创建设备服务实例
	deviceService := service.NewDeviceService()

	// 测试用例
	tests := []struct {
		name              string
		deviceId          string
		cardId            uint32
		cardType          byte
		portNumber        byte
		wantValid         bool
		wantAccountStatus byte
		wantRateMode      byte
		wantBalance       uint32
	}{
		{
			name:              "有效卡片测试",
			deviceId:          "12345678",
			cardId:            123456,
			cardType:          0,
			portNumber:        1,
			wantValid:         true,
			wantAccountStatus: 0x00,
			wantRateMode:      0x00,
			wantBalance:       10000,
		},
		// 可以添加更多测试用例
	}

	// 运行测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValid, gotAccountStatus, gotRateMode, gotBalance := deviceService.ValidateCard(
				tt.deviceId, tt.cardId, tt.cardType, tt.portNumber)

			if gotValid != tt.wantValid {
				t.Errorf("ValidateCard() gotValid = %v, want %v", gotValid, tt.wantValid)
			}
			if gotAccountStatus != tt.wantAccountStatus {
				t.Errorf("ValidateCard() gotAccountStatus = %v, want %v",
					gotAccountStatus, tt.wantAccountStatus)
			}
			if gotRateMode != tt.wantRateMode {
				t.Errorf("ValidateCard() gotRateMode = %v, want %v",
					gotRateMode, tt.wantRateMode)
			}
			if gotBalance != tt.wantBalance {
				t.Errorf("ValidateCard() gotBalance = %v, want %v",
					gotBalance, tt.wantBalance)
			}
		})
	}
}

func TestDeviceService_StartCharging(t *testing.T) {
	// 创建设备服务实例
	deviceService := service.NewDeviceService()

	// 测试用例
	tests := []struct {
		name       string
		deviceId   string
		portNumber byte
		cardId     uint32
		wantErr    bool
	}{
		{
			name:       "开始充电测试",
			deviceId:   "12345678",
			portNumber: 1,
			cardId:     123456,
			wantErr:    false,
		},
		// 可以添加更多测试用例
	}

	// 运行测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrderNumber, err := deviceService.StartCharging(
				tt.deviceId, tt.portNumber, tt.cardId)

			if (err != nil) != tt.wantErr {
				t.Errorf("StartCharging() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && len(gotOrderNumber) == 0 {
				t.Error("StartCharging() returned empty order number")
			}
		})
	}
}
