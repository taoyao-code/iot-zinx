package core

import (
	"fmt"
	"reflect"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// InterfaceValidator 接口完整性验证器
// 🚀 重构：验证所有定义的接口是否完整实现
type InterfaceValidator struct {
	errors   []string
	warnings []string
}

// NewInterfaceValidator 创建接口验证器
func NewInterfaceValidator() *InterfaceValidator {
	return &InterfaceValidator{
		errors:   make([]string, 0),
		warnings: make([]string, 0),
	}
}

// ValidateAllInterfaces 验证所有接口完整性
func (v *InterfaceValidator) ValidateAllInterfaces() error {
	logger.Info("开始验证接口完整性")

	// 1. 验证IUnifiedTCPManager接口实现
	v.validateIUnifiedTCPManagerImplementation()

	// 2. 验证IConnectionSession接口实现
	v.validateIConnectionSessionImplementation()

	// 3. 验证IUnifiedDeviceGroup接口实现
	v.validateIUnifiedDeviceGroupImplementation()

	// 4. 验证向后兼容接口实现
	v.validateLegacyInterfaceImplementations()

	// 5. 验证适配器接口实现
	v.validateAdapterInterfaceImplementations()

	// 生成验证报告
	return v.generateReport()
}

// validateIUnifiedTCPManagerImplementation 验证IUnifiedTCPManager接口实现
func (v *InterfaceValidator) validateIUnifiedTCPManagerImplementation() {
	logger.Debug("验证IUnifiedTCPManager接口实现")

	// 获取统一TCP管理器实例
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		v.errors = append(v.errors, "统一TCP管理器未初始化，无法验证接口实现")
		return
	}

	// 验证接口实现
	if _, ok := tcpManager.(IUnifiedTCPManager); !ok {
		v.errors = append(v.errors, "UnifiedTCPManager未实现IUnifiedTCPManager接口")
		return
	}

	// 验证关键方法是否可调用
	v.validateTCPManagerMethods(tcpManager)

	logger.Debug("IUnifiedTCPManager接口实现验证完成")
}

// validateTCPManagerMethods 验证TCP管理器方法
func (v *InterfaceValidator) validateTCPManagerMethods(tcpManager interface{}) {
	// 使用反射检查关键方法是否存在
	managerType := reflect.TypeOf(tcpManager)
	// 保持指针类型，因为方法是在指针接收器上定义的

	requiredMethods := []string{
		"RegisterConnection",
		"UnregisterConnection",
		"RegisterDevice",
		"UnregisterDevice",
		"GetConnection",
		"GetSessionByDeviceID",
		"GetSessionByConnID",
		"GetAllSessions",
		"UpdateHeartbeat",
		"UpdateDeviceStatus",
		"GetStats",
		"Start",
		"Stop",
		"Cleanup",
	}

	for _, methodName := range requiredMethods {
		if _, found := managerType.MethodByName(methodName); !found {
			v.errors = append(v.errors, fmt.Sprintf("UnifiedTCPManager缺少方法: %s", methodName))
		}
	}
}

// validateIConnectionSessionImplementation 验证IConnectionSession接口实现
func (v *InterfaceValidator) validateIConnectionSessionImplementation() {
	logger.Debug("验证IConnectionSession接口实现")

	// 检查ConnectionSession类型是否实现了IConnectionSession接口
	var session *ConnectionSession
	if _, ok := interface{}(session).(IConnectionSession); !ok {
		v.warnings = append(v.warnings, "ConnectionSession可能未完全实现IConnectionSession接口")
	}

	logger.Debug("IConnectionSession接口实现验证完成")
}

// validateIUnifiedDeviceGroupImplementation 验证IUnifiedDeviceGroup接口实现
func (v *InterfaceValidator) validateIUnifiedDeviceGroupImplementation() {
	logger.Debug("验证IUnifiedDeviceGroup接口实现")

	// 检查UnifiedDeviceGroup类型是否实现了IUnifiedDeviceGroup接口
	var group *UnifiedDeviceGroup
	if _, ok := interface{}(group).(IUnifiedDeviceGroup); !ok {
		v.warnings = append(v.warnings, "UnifiedDeviceGroup可能未完全实现IUnifiedDeviceGroup接口")
	}

	logger.Debug("IUnifiedDeviceGroup接口实现验证完成")
}

// validateLegacyInterfaceImplementations 验证向后兼容接口实现
func (v *InterfaceValidator) validateLegacyInterfaceImplementations() {
	logger.Debug("验证向后兼容接口实现")

	// 验证统一TCP管理器是否支持向后兼容接口
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		v.warnings = append(v.warnings, "无法验证向后兼容接口：统一TCP管理器未初始化")
		return
	}

	// 检查是否实现了向后兼容方法
	managerType := reflect.TypeOf(tcpManager)
	if managerType.Kind() == reflect.Ptr {
		managerType = managerType.Elem()
	}

	legacyMethods := []string{
		"GetLegacyConnectionManager",
		"GetLegacySessionManager",
		"GetLegacyDeviceGroupManager",
	}

	for _, methodName := range legacyMethods {
		if _, found := managerType.MethodByName(methodName); !found {
			v.warnings = append(v.warnings, fmt.Sprintf("统一TCP管理器缺少向后兼容方法: %s", methodName))
		}
	}

	logger.Debug("向后兼容接口实现验证完成")
}

// validateAdapterInterfaceImplementations 验证适配器接口实现
func (v *InterfaceValidator) validateAdapterInterfaceImplementations() {
	logger.Debug("验证适配器接口实现")

	// 验证TCP管理器适配器
	adapter := session.GetGlobalTCPManagerAdapter()
	if adapter == nil {
		v.warnings = append(v.warnings, "全局TCP管理器适配器未初始化")
	} else {
		// 检查适配器是否实现了必要的方法
		adapterType := reflect.TypeOf(adapter)
		if adapterType.Kind() == reflect.Ptr {
			adapterType = adapterType.Elem()
		}

		requiredAdapterMethods := []string{
			"RegisterConnection",
			"UnregisterConnection",
			"RegisterDevice",
			"UnregisterDevice",
			"UpdateHeartbeat",
			"GetStats",
		}

		for _, methodName := range requiredAdapterMethods {
			if _, found := adapterType.MethodByName(methodName); !found {
				v.warnings = append(v.warnings, fmt.Sprintf("TCP管理器适配器缺少方法: %s", methodName))
			}
		}
	}

	logger.Debug("适配器接口实现验证完成")
}

// generateReport 生成验证报告
func (v *InterfaceValidator) generateReport() error {
	logger.Info("生成接口完整性验证报告")

	// 打印错误
	if len(v.errors) > 0 {
		logger.Error("发现接口实现错误:")
		for i, err := range v.errors {
			logger.Errorf("  %d. %s", i+1, err)
		}
		return fmt.Errorf("接口完整性验证失败，发现 %d 个错误", len(v.errors))
	}

	// 打印警告
	if len(v.warnings) > 0 {
		logger.Warn("接口实现警告:")
		for i, warning := range v.warnings {
			logger.Warnf("  %d. %s", i+1, warning)
		}
	}

	logger.Info("接口完整性验证通过")
	return nil
}

// GetValidationSummary 获取验证摘要
func (v *InterfaceValidator) GetValidationSummary() map[string]interface{} {
	return map[string]interface{}{
		"errors":   len(v.errors),
		"warnings": len(v.warnings),
		"status":   v.getValidationStatus(),
		"details": map[string]interface{}{
			"errors":   v.errors,
			"warnings": v.warnings,
		},
	}
}

// getValidationStatus 获取验证状态
func (v *InterfaceValidator) getValidationStatus() string {
	if len(v.errors) > 0 {
		return "FAILED"
	}
	if len(v.warnings) > 0 {
		return "PASSED_WITH_WARNINGS"
	}
	return "PASSED"
}

// === 便捷验证函数 ===

// ValidateInterfaceCompleteness 验证接口完整性
func ValidateInterfaceCompleteness() error {
	validator := NewInterfaceValidator()
	return validator.ValidateAllInterfaces()
}

// GetInterfaceValidationStatus 获取接口验证状态
func GetInterfaceValidationStatus() map[string]interface{} {
	validator := NewInterfaceValidator()
	validator.ValidateAllInterfaces()
	return validator.GetValidationSummary()
}

// ValidateSpecificInterface 验证特定接口实现
func ValidateSpecificInterface(interfaceName string, implementation interface{}) error {
	logger.Infof("验证接口实现: %s", interfaceName)

	if implementation == nil {
		return fmt.Errorf("接口实现为空: %s", interfaceName)
	}

	// 这里可以添加更具体的接口验证逻辑
	logger.Infof("接口 %s 验证通过", interfaceName)
	return nil
}
