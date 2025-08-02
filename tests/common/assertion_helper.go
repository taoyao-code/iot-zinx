package common

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// AssertionHelper 测试断言辅助工具
type AssertionHelper struct{}

// NewAssertionHelper 创建断言辅助工具实例
func NewAssertionHelper() *AssertionHelper {
	return &AssertionHelper{}
}

// AssertTCPResponse 断言TCP响应
func (ah *AssertionHelper) AssertTCPResponse(t *testing.T, response []byte, expectedMinLength int, description string) {
	t.Helper()

	if response == nil {
		t.Errorf("%s: 响应为空", description)
		return
	}

	if len(response) < expectedMinLength {
		t.Errorf("%s: 响应长度不足，期望至少%d字节，实际%d字节", description, expectedMinLength, len(response))
		return
	}

	t.Logf("%s: 响应成功，长度%d字节，内容: %s", description, len(response), hex.EncodeToString(response))
}

// AssertTCPResponseEquals 断言TCP响应内容相等
func (ah *AssertionHelper) AssertTCPResponseEquals(t *testing.T, actual, expected []byte, description string) {
	t.Helper()

	if !bytes.Equal(actual, expected) {
		t.Errorf("%s: 响应内容不匹配\n期望: %s\n实际: %s",
			description,
			hex.EncodeToString(expected),
			hex.EncodeToString(actual))
		return
	}

	t.Logf("%s: 响应内容匹配", description)
}

// AssertTCPResponseContains 断言TCP响应包含特定内容
func (ah *AssertionHelper) AssertTCPResponseContains(t *testing.T, response []byte, expectedContent []byte, description string) {
	t.Helper()

	if !bytes.Contains(response, expectedContent) {
		t.Errorf("%s: 响应不包含期望内容\n响应: %s\n期望包含: %s",
			description,
			hex.EncodeToString(response),
			hex.EncodeToString(expectedContent))
		return
	}

	t.Logf("%s: 响应包含期望内容", description)
}

// AssertHTTPStatus 断言HTTP状态码
func (ah *AssertionHelper) AssertHTTPStatus(t *testing.T, resp *http.Response, expectedStatus int, description string) {
	t.Helper()

	if resp == nil {
		t.Errorf("%s: HTTP响应为空", description)
		return
	}

	if resp.StatusCode != expectedStatus {
		t.Errorf("%s: HTTP状态码不匹配，期望%d，实际%d", description, expectedStatus, resp.StatusCode)
		return
	}

	t.Logf("%s: HTTP状态码正确(%d)", description, resp.StatusCode)
}

// AssertHTTPStatusRange 断言HTTP状态码在指定范围内
func (ah *AssertionHelper) AssertHTTPStatusRange(t *testing.T, resp *http.Response, minStatus, maxStatus int, description string) {
	t.Helper()

	if resp == nil {
		t.Errorf("%s: HTTP响应为空", description)
		return
	}

	if resp.StatusCode < minStatus || resp.StatusCode > maxStatus {
		t.Errorf("%s: HTTP状态码超出范围，期望%d-%d，实际%d", description, minStatus, maxStatus, resp.StatusCode)
		return
	}

	t.Logf("%s: HTTP状态码在范围内(%d)", description, resp.StatusCode)
}

// AssertJSONResponse 断言JSON响应格式
func (ah *AssertionHelper) AssertJSONResponse(t *testing.T, body []byte, expectedFields []string, description string) {
	t.Helper()

	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		t.Errorf("%s: JSON解析失败: %v", description, err)
		return
	}

	for _, field := range expectedFields {
		if _, exists := jsonData[field]; !exists {
			t.Errorf("%s: JSON响应缺少字段'%s'", description, field)
		}
	}

	t.Logf("%s: JSON响应格式正确", description)
}

// AssertDeviceState 断言设备状态
func (ah *AssertionHelper) AssertDeviceState(t *testing.T, suite *TestSuite, deviceID, expectedState string) {
	t.Helper()

	actualState, exists := suite.GetDeviceState(deviceID)
	if !exists {
		t.Errorf("设备%s状态不存在", deviceID)
		return
	}

	if actualState != expectedState {
		t.Errorf("设备%s状态不匹配，期望'%s'，实际'%s'", deviceID, expectedState, actualState)
		return
	}

	t.Logf("设备%s状态正确: %s", deviceID, actualState)
}

// AssertNoError 断言无错误
func (ah *AssertionHelper) AssertNoError(t *testing.T, err error, description string) {
	t.Helper()

	if err != nil {
		t.Errorf("%s: 期望无错误，实际错误: %v", description, err)
		return
	}

	t.Logf("%s: 无错误", description)
}

// AssertError 断言有错误
func (ah *AssertionHelper) AssertError(t *testing.T, err error, description string) {
	t.Helper()

	if err == nil {
		t.Errorf("%s: 期望有错误，但实际无错误", description)
		return
	}

	t.Logf("%s: 正确产生错误: %v", description, err)
}

// AssertErrorContains 断言错误包含特定信息
func (ah *AssertionHelper) AssertErrorContains(t *testing.T, err error, expectedMessage string, description string) {
	t.Helper()

	if err == nil {
		t.Errorf("%s: 期望有错误，但实际无错误", description)
		return
	}

	if !strings.Contains(err.Error(), expectedMessage) {
		t.Errorf("%s: 错误信息不包含期望内容\n错误: %v\n期望包含: %s", description, err, expectedMessage)
		return
	}

	t.Logf("%s: 错误信息包含期望内容", description)
}

// AssertDuration 断言执行时间
func (ah *AssertionHelper) AssertDuration(t *testing.T, duration, maxDuration time.Duration, description string) {
	t.Helper()

	if duration > maxDuration {
		t.Errorf("%s: 执行时间过长，期望不超过%v，实际%v", description, maxDuration, duration)
		return
	}

	t.Logf("%s: 执行时间正常(%v)", description, duration)
}

// AssertConcurrentResults 断言并发测试结果
func (ah *AssertionHelper) AssertConcurrentResults(t *testing.T, successCount, errorCount int64, totalCount int, minSuccessRate float64, description string) {
	t.Helper()

	total := successCount + errorCount
	if total != int64(totalCount) {
		t.Errorf("%s: 总数不匹配，期望%d，实际%d", description, totalCount, total)
	}

	successRate := float64(successCount) / float64(total) * 100
	if successRate < minSuccessRate {
		t.Errorf("%s: 成功率过低，期望至少%.1f%%，实际%.1f%%", description, minSuccessRate, successRate)
		return
	}

	t.Logf("%s: 并发测试成功，成功率%.1f%% (%d/%d)", description, successRate, successCount, total)
}

// AssertProtocolPacket 断言协议包格式
func (ah *AssertionHelper) AssertProtocolPacket(t *testing.T, packet []byte, description string) {
	t.Helper()

	helper := NewProtocolHelper()
	if err := helper.ValidateProtocolPacket(packet); err != nil {
		t.Errorf("%s: 协议包格式错误: %v", description, err)
		return
	}

	t.Logf("%s: 协议包格式正确", description)
}

// AssertEqual 通用相等断言
func (ah *AssertionHelper) AssertEqual(t *testing.T, actual, expected interface{}, description string) {
	t.Helper()

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("%s: 值不相等\n期望: %v\n实际: %v", description, expected, actual)
		return
	}

	t.Logf("%s: 值相等", description)
}

// AssertNotEqual 通用不相等断言
func (ah *AssertionHelper) AssertNotEqual(t *testing.T, actual, expected interface{}, description string) {
	t.Helper()

	if reflect.DeepEqual(actual, expected) {
		t.Errorf("%s: 值不应该相等，但实际相等: %v", description, actual)
		return
	}

	t.Logf("%s: 值不相等", description)
}

// AssertTrue 断言为真
func (ah *AssertionHelper) AssertTrue(t *testing.T, condition bool, description string) {
	t.Helper()

	if !condition {
		t.Errorf("%s: 期望为真，但实际为假", description)
		return
	}

	t.Logf("%s: 条件为真", description)
}

// AssertFalse 断言为假
func (ah *AssertionHelper) AssertFalse(t *testing.T, condition bool, description string) {
	t.Helper()

	if condition {
		t.Errorf("%s: 期望为假，但实际为真", description)
		return
	}

	t.Logf("%s: 条件为假", description)
}

// AssertTestSuiteHealth 断言测试套件健康状态
func (ah *AssertionHelper) AssertTestSuiteHealth(t *testing.T, suite *TestSuite, description string) {
	t.Helper()

	if !suite.IsHealthy() {
		stats := suite.GetTestStatistics()
		t.Errorf("%s: 测试套件不健康，成功率%.2f%%", description, stats["success_rate"])
		return
	}

	t.Logf("%s: 测试套件健康", description)
}

// LogTestResult 记录测试结果（用于调试）
func (ah *AssertionHelper) LogTestResult(t *testing.T, result TestResult) {
	t.Helper()

	status := "✅"
	if !result.Success {
		status = "❌"
	}

	t.Logf("%s %s (%s) - %v - %s",
		status,
		result.TestName,
		result.TestType,
		result.Duration,
		result.Description)

	if result.Error != nil {
		t.Logf("   错误: %v", result.Error)
	}
}

// 全局断言辅助工具实例
var DefaultAssertionHelper = NewAssertionHelper()
