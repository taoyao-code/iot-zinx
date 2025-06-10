# 任务：修复 TCP 初始读取超时问题

## 任务描述

诊断并修复在 TCP 连接建立大约 30 秒后发生的 "i/o timeout" 错误。初步假设是新连接启动时设置的 `initialReadDeadline` 在客户端发送第一个数据包（如 ICCID 或注册信息）后未被正确清除或更新。

## 已完成

1.  **初步分析:**
    - 审查了显示 "i/o timeout" 的服务器日志。
    - 将 `InitialReadDeadlineSeconds`（在 `gateway.yaml` 中配置为 30 秒，并在 `connection_hooks.go` 中用作默认值）确定为如果未重置则可能的原因。
2.  **代码审查与调查:**
    - 阅读 `pkg/network/connection_hooks.go`: 确认 `OnConnectionStart` 使用来自 `configs/gateway.yaml` 的 `initialReadDeadlineSeconds` (或 30 秒默认值) 设置初始读取截止时间。在此文件中未找到稍后更新此截止时间的逻辑。
    - 阅读 `configs/gateway.yaml`: 确认 `initialReadDeadlineSeconds: 30` 和 `defaultReadDeadlineSeconds: 90`。
    - 阅读 `conf/zinx.json`: 基本 Zinx 配置，与所讨论的超时值无直接关系。
    - 使用语义搜索查找 "设备注册处理, ICCID 处理, DNYFrameHandlerBase HandleFrame, CreateOrUpdateSession"。
    - 发现 `internal/infrastructure/zinx_server/handlers/sim_card_handler.go` 和 `internal/infrastructure/zinx_server/handlers/device_register_handler.go` *已包含*在处理 ICCID 或设备注册后将 TCP 读取截止时间更新为 `defaultReadDeadlineSeconds` 的逻辑。这意味着问题可能是这些处理器未被调用于超时的连接，或者存在其他问题。
3.  **日志增强:**
    - 为了检查 `SimCardHandler` 是否被调用，在其 `Handle` 方法中添加了入口日志。
4.  **根本原因定位与修复:**
    - 通过分析用户提供的日志，确认 `SimCardHandler` 未被调用。
    - 进一步分析代码路径：
      - `pkg/protocol/dny_decoder.go` 中的 `DNY_Decoder.Intercept` 方法调用 `decodedFrame.GetMsgID()` 来设置 Zinx `iMessage` 的 `MsgID`。
      - `pkg/protocol/dny_types.go` 中的 `DecodedDNYFrame.GetMsgID()` 方法先前为 `FrameTypeICCID` 类型的帧返回 `0x1001`。
      - `internal/infrastructure/zinx_server/handlers/router.go` 中 `SimCardHandler` 注册的 `MsgID` 是 `0xFF01`。
      - `pkg/protocol/dny_protocol_parser.go` 文件中定义了常量 `MSG_ID_ICCID = 0xFF01`，表明项目内部期望 ICCID 使用 `0xFF01` 作为消息 ID。
    - **结论**: `MsgID` 不匹配 (`0x1001` 在 `dny_types.go` vs `0xFF01` 在 `router.go` 和 `dny_protocol_parser.go`) 导致 `SimCardHandler` 未被调用。
    - **修复**: 修改 `pkg/protocol/dny_types.go` 中 `DecodedDNYFrame.GetMsgID()` 方法，当 `FrameType` 为 `FrameTypeICCID` 时，返回 `0xFF01`，与项目中其他定义保持一致。
5.  **修复 `repeated api` Panic:**
    - **问题**: 服务启动时出现 `panic: repeated api , msgID = 65282` (0xFF02)。
    - **原因**:
      - 在 `internal/infrastructure/zinx_server/handlers/router.go` 中，`SimCardHandler` 被错误地注册到了 `protocol.MSG_ID_HEARTBEAT` (值为 `0xFF02`)。
      - 同时，`LinkHeartbeatHandler` 也被注册到了硬编码的 `0xFF02`。
      - 这导致了 `MsgID 0xFF02` 的重复注册。
    - **分析**:
      - `SimCardHandler` 应该处理 ICCID 上报，对应的 `MsgID` 是 `protocol.MSG_ID_ICCID` (`0xFF01`)。
      - `LinkHeartbeatHandler` 应该处理 `link` 心跳，对应的 `MsgID` 是 `protocol.MSG_ID_HEARTBEAT` (`0xFF02`)。
      - 在 `pkg/protocol/dny_types.go` 的 `GetMsgID()` 中，`FrameTypeLinkHeartbeat` 返回的是 `0x1002`，而不是 `0xFF02`。
    - **修复步骤**:
      - **步骤 1 (已由用户手动完成)**: 修改 `internal/infrastructure/zinx_server/handlers/router.go`，将 `SimCardHandler` 的注册从 `protocol.MSG_ID_HEARTBEAT` 改为 `protocol.MSG_ID_ICCID`。
        ```go
        // server.AddRouter(protocol.MSG_ID_HEARTBEAT, &SimCardHandler{}) // 旧的错误注册
        server.AddRouter(protocol.MSG_ID_ICCID, &SimCardHandler{}) // 正确的注册
        ```
      - **步骤 2**: 修改 `pkg/protocol/dny_types.go` 中的 `GetMsgID()` 方法，使 `FrameTypeLinkHeartbeat` 返回 `0xFF02` (即 `protocol.MSG_ID_HEARTBEAT`)，以确保解码器生成的心跳 `MsgID` 与路由注册一致。

## 待处理

1.  使用 `SimCardHandler` 中的新日志记录来测试应用程序，以查看它是否被调用于正在超时的连接。
2.  根据测试结果:
    - 如果 `SimCardHandler` (或用于注册数据包的 `DeviceRegisterHandler`) 未被调用：调查原因 (例如，路由问题，数据格式在到达这些处理器之前与预期不符)。
    - 如果处理器被调用但超时仍然发生：重新评估。也许 `defaultReadDeadlineSeconds` (90 秒) 对于后续操作也太短，或者截止时间在其他地方被错误地重置，或者问题与这些特定的读取截止时间无关。
3.  根据诊断结果实施明确的修复。
4.  彻底测试修复。
5.  使用发现和解决方案更新任务 markdown 文件 (`./issues/修复TCP初始读取超时问题.md`)。

## 当前状态

主要疑点仍是读取超时，但现在更关注的是为什么 `SimCardHandler` 或 `DeviceRegisterHandler` 中现有的更新机制可能对失败的连接无效，或者是否存在其他超时。

## 代码状态

- `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/pkg/network/connection_hooks.go` (已审查)
- `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/conf/zinx.json` (已审查)
- `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/configs/gateway.yaml` (已审查, `initialReadDeadlineSeconds` 是活动选择)
- `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/internal/infrastructure/zinx_server/handlers/sim_card_handler.go` (已审查, 已修改)
- `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/internal/infrastructure/zinx_server/handlers/device_register_handler.go` (通过搜索审查，已知包含截止时间更新逻辑)
- `./issues/修复TCP初始读取超时问题.md` (已创建)

## 变更

- 在 `/Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx/internal/infrastructure/zinx_server/handlers/sim_card_handler.go` 中:

  - 向 `Handle` 方法添加了入口日志:

    ```go
    // ...
    func (h *SimCardHandler) Handle(request ziface.IRequest) {
        conn := request.GetConnection()
        data := request.GetData()

        logger.WithFields(logrus.Fields{ // 添加入口日志
            "connID":     conn.GetConnID(),
            "remoteAddr": conn.RemoteAddr().String(),
            "dataLen":    len(data),
            "dataHex":    fmt.Sprintf("%x", data),
        }).Info("SimCardHandler: Handle method called")

        // 确保数据是有效的SIM卡号 (支持标准ICCID长度范围: 19-25字节)
    // ...
    ```

  - 修改 `DecodedDNYFrame.GetMsgID()` 方法以修复 `MsgID` 不匹配问题:

    ```go
    // ...
    func (f *DecodedDNYFrame) GetMsgID() uint32 {
        if f.FrameType == FrameTypeICCID {
            return 0xFF01
        }
        return f.MsgID
    }
    // ...
    ```

- 在 `pkg/protocol/dny_types.go` 中:

  - 修改 `GetMsgID()` 方法，使 `FrameTypeLinkHeartbeat` 返回 `0xFF02` (即 `protocol.MSG_ID_HEARTBEAT`):

    ```go
    // ...
    func (f *DecodedDNYFrame) GetMsgID() uint32 {
        switch f.FrameType {
        case FrameTypeICCID:
            return 0xFF01
        case FrameTypeLinkHeartbeat:
            return 0xFF02 // 修复心跳帧MsgID
        }
        return f.MsgID
    }
    // ...
    ```

## 依赖项 (基于代码上下文隐式推断)

- Zinx 框架 (`github.com/aceld/zinx`)
- Logrus (`github.com/sirupsen/logrus`)
- 用于配置、日志记录、网络协议、会话管理的内部项目包。
