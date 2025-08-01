package constants

// 简化架构下的命令字定义
const (
	CmdRegisterDevice  = 0x01
	CmdDeviceHeartbeat = 0x21
	CmdStartCharging   = 0x11
	CmdStopCharging    = 0x12
	CmdReportStatus    = 0x31
)

// 简化响应状态
const (
	RespStatusOK    = 0x00
	RespStatusError = 0x01
)
