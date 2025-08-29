# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

IoT-Zinx is a TCP gateway system for charging device management based on the Zinx network framework. It implements hexagonal architecture (ports and adapters) for communication with charging pile devices, handling device registration, heartbeat management, charging control, and real-time status monitoring.

## Build & Development Commands

### Build Commands
- `make build-all` - Build all components (gateway, client, server-api, dny-parser)
- `make build-gateway` - Build only the gateway component
- `make build-client` - Build only the client component  
- `make build-server-api` - Build only the server-api component
- `make build-dny-parser` - Build only the DNY parser tool
- `make build-multi-platform` - Build for multiple platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)

### Development Commands
- `make test` - Run all tests
- `make lint` - Run code linting with golangci-lint
- `make fmt` - Format code with go fmt
- `make cover` - Generate test coverage report
- `make tidy` - Tidy go.mod dependencies
- `make swagger` - Generate Swagger API documentation
- `make clean` - Clean build artifacts

### Run Commands
- `make run-gateway` - Run the gateway component directly
- `make run-client` - Run the client component
- `make run-server-api` - Run the server API component
- `make run-dny-parser` - Run the DNY parser tool
- `./bin/iot_gateway --config configs/gateway.yaml` - Run built gateway with config

## Core Architecture

### Hexagonal Architecture Layers
- **Domain Layer** (`internal/domain/`): Core business models and DNY protocol definitions
- **Application Layer** (`internal/app/`): Business service implementations  
- **Ports Layer** (`internal/ports/`): System interfaces (TCP server, HTTP server, heartbeat manager)
- **Infrastructure Layer** (`internal/infrastructure/`): Technical implementations (Zinx handlers, Redis, config, logging)
- **Adapters Layer** (`internal/adapter/`): External system integrations (HTTP handlers)

### Key Components

#### Unified Data Management (`pkg/core/`)
- **TCPManager**: Central device and connection manager - single source of truth
- **Device struct**: Unified device information storage (DeviceID, PhysicalID, ICCID, Status, LastHeartbeat)
- **ConnectionSession**: Connection-level data management
- Thread-safe operations with proper mutex protection

#### Protocol Handling (`pkg/protocol/`)
- **DNY Protocol**: Complete support for device communication protocol
- **Command Types**: 0x01 (heartbeat), 0x20 (registration), 0x82 (charge control), 0x81 (device status), etc.
- **Packet Parser**: Unified DNY packet parsing and building

#### Network Layer (`pkg/network/`)
- **Unified Sender**: Synchronous/asynchronous message sending
- **TCP Writer**: Centralized TCP communication channel
- **Connection Health**: Automatic connection monitoring and cleanup

### Device Lifecycle
1. **Connection**: Device establishes TCP connection, gets ConnectionSession with ConnID
2. **ICCID Recognition**: Device sends SIM card number for initial identification  
3. **Registration**: Device sends 0x20 registration request, creates Device object
4. **Heartbeat**: Multiple heartbeat types (standard 0x01, main 0x11, link, etc.) update Device.LastHeartbeat
5. **Business Operations**: Charging control, status queries through unified DeviceGateway interface
6. **Cleanup**: Automatic detection of offline devices and connection cleanup

## Smart Charging System

### Dynamic Power Control
- Located in `pkg/gateway/dynamic_power_controller.go`
- Uses 0x82 "overload power" field for real-time power adjustment
- Based on 0x06/0x26 heartbeat power readings and device status
- Gradual power reduction until reaching minPowerW or device full

### Configuration (`configs/gateway.yaml`)
```yaml
smartCharging:
  enabled: true
  stepPercent: 0.1
  stepIntervalSeconds: 180
  peakHoldSeconds: 300
  minPowerW: 80
```

## Device ID Handling

The system supports automatic conversion between decimal and hexadecimal DeviceID formats:
- **Decimal format**: Standard numeric IDs
- **Hexadecimal format**: Used by some device types
- **PhysicalID**: Device hardware identifier, may differ from DeviceID
- All conversions handled automatically in `pkg/utils/device_id_processor.go`

## Logging System

Enhanced logging with logrus + lumberjack:
- **Structured logging**: JSON format with contextual fields
- **Automatic rotation**: Size-based log file rotation
- **Multiple outputs**: Console and file output support
- **Zinx integration**: Unified logging for framework messages
- **Configuration**: Set via `configs/gateway.yaml` logger section

## Testing

- Test files located in `/test/` directory
- Handler-specific tests in `internal/infrastructure/zinx_server/handlers/*_test.go`
- Run with `make test` for full test suite
- Coverage reports with `make cover`

## Key APIs

### Device Operations (via TCPManager)
```go
// Get global TCP manager
tcpManager := core.GetGlobalTCPManager()

// Device registration
err := tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)

// Get device info (single source of truth)
device, exists := tcpManager.GetDeviceByID(deviceID)

// Update heartbeat (unified interface)
tcpManager.UpdateHeartbeat(deviceID)

// Check device status
isOnline := tcpManager.IsDeviceOnline(deviceID)
```

### Charging Control
```go
// Start charging
gateway.StartCharging(deviceID, powerW, timeMinutes)

// Stop charging  
gateway.StopCharging(deviceID)

// Update charging power (dynamic control)
gateway.UpdateChargingPower(deviceID, newPowerW)
```

## Important Notes

- **Single Data Source**: Always use TCPManager.GetDeviceByID() for device information
- **Thread Safety**: All device operations are mutex-protected
- **Error Handling**: Comprehensive error handling with structured logging
- **Memory Optimization**: Eliminated duplicate data storage, 50%+ memory reduction
- **Protocol Compliance**: Full DNY protocol implementation with proper packet validation