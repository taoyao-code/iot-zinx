package utils

import (
	"testing"
)

func TestFormatPhysicalID(t *testing.T) {
	tests := []struct {
		name       string
		physicalID uint32
		expected   string
	}{
		{
			name:       "Standard device ID",
			physicalID: 0x04A26CF3,
			expected:   "04A26CF3",
		},
		{
			name:       "Zero device ID",
			physicalID: 0x00000000,
			expected:   "00000000",
		},
		{
			name:       "Max device ID",
			physicalID: 0xFFFFFFFF,
			expected:   "FFFFFFFF",
		},
		{
			name:       "Small device ID",
			physicalID: 0x00000001,
			expected:   "00000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPhysicalID(tt.physicalID)
			if result != tt.expected {
				t.Errorf("FormatPhysicalID(%d) = %s, want %s", tt.physicalID, result, tt.expected)
			}
		})
	}
}

func TestParsePhysicalID(t *testing.T) {
	tests := []struct {
		name        string
		deviceIDStr string
		expected    uint32
		expectError bool
	}{
		{
			name:        "Valid hex string",
			deviceIDStr: "04A26CF3",
			expected:    0x04A26CF3,
			expectError: false,
		},
		{
			name:        "Valid hex string with lowercase",
			deviceIDStr: "04a26cf3",
			expected:    0x04A26CF3,
			expectError: false,
		},
		{
			name:        "Valid hex string with spaces",
			deviceIDStr: " 04A26CF3 ",
			expected:    0x04A26CF3,
			expectError: false,
		},
		{
			name:        "Invalid hex string",
			deviceIDStr: "INVALID",
			expected:    0,
			expectError: true,
		},
		{
			name:        "Empty string",
			deviceIDStr: "",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParsePhysicalID(tt.deviceIDStr)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParsePhysicalID(%s) expected error, got nil", tt.deviceIDStr)
				}
			} else {
				if err != nil {
					t.Errorf("ParsePhysicalID(%s) unexpected error: %v", tt.deviceIDStr, err)
				}
				if result != tt.expected {
					t.Errorf("ParsePhysicalID(%s) = %d, want %d", tt.deviceIDStr, result, tt.expected)
				}
			}
		})
	}
}

func TestFormatCardNumber(t *testing.T) {
	tests := []struct {
		name     string
		cardID   uint32
		expected string
	}{
		{
			name:     "Standard card ID",
			cardID:   0x12345678,
			expected: "12345678",
		},
		{
			name:     "Zero card ID",
			cardID:   0x00000000,
			expected: "00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCardNumber(tt.cardID)
			if result != tt.expected {
				t.Errorf("FormatCardNumber(%d) = %s, want %s", tt.cardID, result, tt.expected)
			}
		})
	}
}

func TestFormatDeviceIDForDisplay(t *testing.T) {
	tests := []struct {
		name       string
		physicalID uint32
		expected   string
	}{
		{
			name:       "Device ID 04A26CF3 to decimal",
			physicalID: 0x04A26CF3,
			expected:   "77753587",
		},
		{
			name:       "Device ID 04A228CD to decimal",
			physicalID: 0x04A228CD,
			expected:   "77736141",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDeviceIDForDisplay(tt.physicalID)
			if result != tt.expected {
				t.Errorf("FormatDeviceIDForDisplay(%d) = %s, want %s", tt.physicalID, result, tt.expected)
			}
		})
	}
}

func TestValidateDeviceID(t *testing.T) {
	tests := []struct {
		name        string
		deviceIDStr string
		expectError bool
	}{
		{
			name:        "Valid device ID",
			deviceIDStr: "04A26CF3",
			expectError: false,
		},
		{
			name:        "Empty device ID",
			deviceIDStr: "",
			expectError: true,
		},
		{
			name:        "Invalid device ID",
			deviceIDStr: "INVALID",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeviceID(tt.deviceIDStr)
			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateDeviceID(%s) expected error, got nil", tt.deviceIDStr)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDeviceID(%s) unexpected error: %v", tt.deviceIDStr, err)
				}
			}
		})
	}
}

func TestIsValidPhysicalID(t *testing.T) {
	tests := []struct {
		name       string
		physicalID uint32
		expected   bool
	}{
		{
			name:       "Valid physical ID",
			physicalID: 0x04A26CF3,
			expected:   true,
		},
		{
			name:       "Invalid physical ID (zero)",
			physicalID: 0x00000000,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidPhysicalID(tt.physicalID)
			if result != tt.expected {
				t.Errorf("IsValidPhysicalID(%d) = %t, want %t", tt.physicalID, result, tt.expected)
			}
		})
	}
}

func TestGetDeviceIDInfo(t *testing.T) {
	physicalID := uint32(0x04A26CF3)
	info := GetDeviceIDInfo(physicalID)

	if info.PhysicalID != physicalID {
		t.Errorf("GetDeviceIDInfo().PhysicalID = %d, want %d", info.PhysicalID, physicalID)
	}

	expectedHex := "04A26CF3"
	if info.HexString != expectedHex {
		t.Errorf("GetDeviceIDInfo().HexString = %s, want %s", info.HexString, expectedHex)
	}

	expectedDecimal := "77753587"
	if info.DecimalString != expectedDecimal {
		t.Errorf("GetDeviceIDInfo().DecimalString = %s, want %s", info.DecimalString, expectedDecimal)
	}
}

func TestDeviceIDFormatter(t *testing.T) {
	formatter := NewDeviceIDFormatter()
	physicalID := uint32(0x04A26CF3)
	expected := "04A26CF3"

	result := formatter.FormatPhysicalID(physicalID)
	if result != expected {
		t.Errorf("DeviceIDFormatter.FormatPhysicalID(%d) = %s, want %s", physicalID, result, expected)
	}
}

func TestDefaultFormatter(t *testing.T) {
	physicalID := uint32(0x04A26CF3)
	expected := "04A26CF3"

	result := FormatDeviceID(physicalID)
	if result != expected {
		t.Errorf("FormatDeviceID(%d) = %s, want %s", physicalID, result, expected)
	}
}
