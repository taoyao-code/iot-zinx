```mermaid
graph TB
    %% HTTP API层
    subgraph "HTTP API层"
        API1["/api/v1/device/{id}/status<br/>设备状态查询"]
        API2["/api/v1/device/command<br/>设备控制命令"]
        API3["/api/v1/device/{id}/detail<br/>设备详情查询"]
        API4["/api/v1/device/locate<br/>设备定位命令"]
    end

    %% 业务网关层
    subgraph "业务网关层"
        GW1[DeviceGateway<br/>设备网关统一接口]
        GW2[智能DeviceID处理器<br/>支持多种格式转换]
        GW3[命令发送接口<br/>SendCommandToDevice]
        GW4[设备状态查询接口<br/>IsDeviceOnline]
    end

    %% 核心管理层
    subgraph "核心管理层"
        MGR1[TCPManager<br/>核心TCP连接管理器]
        MGR2[三层映射管理<br/>connections/deviceGroups/deviceIndex]
        MGR3[设备注册管理<br/>RegisterDevice]
        MGR4[心跳管理<br/>UpdateHeartbeat]
        MGR5[连接生命周期管理<br/>Register/Unregister]
    end

    %% 数据存储层
    subgraph "数据存储层"
        subgraph "连接管理"
            DS1[(connections<br/>connID → ConnectionSession)]
            DS1_1[ConnectionSession<br/>• SessionID<br/>• ConnID<br/>• RemoteAddr<br/>• LastActivity<br/>• ConnectionState]
        end

        subgraph "设备组管理"
            DS2[(deviceGroups<br/>iccid → DeviceGroup)]
            DS2_1[DeviceGroup<br/>• ICCID<br/>• ConnID<br/>• Connection<br/>• Devices Map<br/>• LastActivity]
        end

        subgraph "设备信息管理"
            DS3[Device<br/>• DeviceID<br/>• PhysicalID<br/>• ICCID<br/>• DeviceType<br/>• Status<br/>• LastHeartbeat<br/>• Properties<br/>• mutex保护]
        end

        subgraph "索引映射"
            DS4[(deviceIndex<br/>deviceID → iccid)]
        end
    end

    %% 网络传输层
    subgraph "网络传输层"
        NET1[Zinx框架<br/>TCP服务器]
        NET2[DNYDecoder<br/>协议解析拦截器]
        NET3[TCPWriter<br/>统一发送通道]
        NET4[连接钩子<br/>OnConnStart/OnConnStop]
    end

    %% 协议处理层
    subgraph "协议处理层"
        PROTO1[SimCardHandler<br/>ICCID处理]
        PROTO2[DeviceRegisterHandler<br/>设备注册]
        PROTO3[HeartbeatHandler<br/>心跳处理]
        PROTO4[PowerHeartbeatHandler<br/>功率心跳]
        PROTO5[SettlementHandler<br/>结算处理]
        PROTO6[其他业务Handler]
    end

    %% 数据关系
    DS1 --> DS1_1
    DS2 --> DS2_1
    DS2_1 --> DS3

    %% 连接关系
    API1 --> GW1
    API2 --> GW1
    API3 --> GW1
    API4 --> GW1

    GW1 --> GW2
    GW1 --> GW3
    GW1 --> GW4

    GW2 --> MGR1
    GW3 --> MGR1
    GW4 --> MGR1

    MGR1 --> MGR2
    MGR1 --> MGR3
    MGR1 --> MGR4
    MGR1 --> MGR5

    MGR2 --> DS1
    MGR2 --> DS2
    MGR2 --> DS4
    MGR3 --> DS3
    MGR4 --> DS3

    NET1 --> NET2
    NET1 --> NET4
    NET2 --> PROTO1
    NET2 --> PROTO2
    NET2 --> PROTO3
    NET2 --> PROTO4
    NET2 --> PROTO5
    NET2 --> PROTO6

    PROTO1 --> MGR1
    PROTO2 --> MGR1
    PROTO3 --> MGR1
    PROTO4 --> MGR1
    PROTO5 --> MGR1
    PROTO6 --> MGR1

    GW3 --> NET3

    %% 关键特性标注
    subgraph "关键特性"
        FEAT1[✅ 单一数据源<br/>Device作为设备信息唯一来源]
        FEAT2[✅ 职责分离<br/>ConnectionSession管理连接<br/>Device管理设备信息]
        FEAT3[✅ 并发安全<br/>Device结构mutex保护]
        FEAT4[✅ 统一接口<br/>所有模块通过TCPManager访问数据]
        FEAT5[✅ 智能转换<br/>支持多种DeviceID格式]
    end

    %% 样式定义
    classDef apiLayer fill:#e3f2fd
    classDef gatewayLayer fill:#f3e5f5
    classDef coreLayer fill:#e8f5e8
    classDef dataLayer fill:#fff3e0
    classDef networkLayer fill:#fce4ec
    classDef protoLayer fill:#f1f8e9
    classDef featureBox fill:#f5f5f5,stroke:#666,stroke-width:2px

    class API1,API2,API3,API4 apiLayer
    class GW1,GW2,GW3,GW4 gatewayLayer
    class MGR1,MGR2,MGR3,MGR4,MGR5 coreLayer
    class DS1,DS1_1,DS2,DS2_1,DS3,DS4 dataLayer
    class NET1,NET2,NET3,NET4 networkLayer
    class PROTO1,PROTO2,PROTO3,PROTO4,PROTO5,PROTO6 protoLayer
    class FEAT1,FEAT2,FEAT3,FEAT4,FEAT5 featureBox
```

```mermaid
graph TD
    %% TCP连接层面链路
    subgraph "TCP连接管理链路"
        A1[TCP连接建立] --> A2[OnConnStart回调]
        A2 --> A3[tcpManager.RegisterConnection]
        A3 --> A4[ConnectionSession创建]
        A4 --> A5[连接属性设置]
        A5 --> A6[连接状态：已连接]

        B1[连接断开] --> B2[OnConnStop回调]
        B2 --> B3[tcpManager.UnregisterConnection]
        B3 --> B4[资源清理]
        B4 --> B5[设备状态更新]
        B5 --> B6[连接状态：已断开]
    end

    %% ICCID处理链路
    subgraph "ICCID处理链路"
        C1[ICCID数据接收] --> C2[SimCardHandler验证]
        C2 --> C3{ICCID格式验证}
        C3 -->|有效| C4[连接属性存储]
        C3 -->|无效| C5[拒绝处理]
        C4 --> C6[TCPManager同步]
        C6 --> C7[DeviceGroup准备]
        C7 --> C8[等待设备注册]
    end

    %% 设备注册链路
    subgraph "设备注册链路"
        D1[设备注册包接收] --> D2[DeviceRegisterHandler处理]
        D2 --> D3[设备ID解析]
        D3 --> D4[PhysicalID转换]
        D4 --> D5{ICCID验证}
        D5 -->|存在| D6[Device创建]
        D5 -->|不存在| D7[注册失败]
        D6 --> D8[DeviceGroup关联]
        D8 --> D9[设备索引映射<br/>deviceID→iccid]
        D9 --> D10[注册成功响应]
    end

    %% 心跳处理链路
    subgraph "心跳处理链路"
        E1[心跳包接收] --> E2[各Handler处理<br/>HeartbeatHandler<br/>MainHeartbeatHandler<br/>PowerHeartbeatHandler]
        E2 --> E3[设备ID提取]
        E3 --> E4[tcpManager.UpdateHeartbeat]
        E4 --> E5[Device.LastHeartbeat更新]
        E5 --> E6[连接会话活动时间更新]
        E6 --> E7[设备状态：在线]
    end

    %% API数据获取链路
    subgraph "API数据获取链路"
        F1[API请求] --> F2[智能DeviceID处理<br/>支持十进制/十六进制]
        F2 --> F3[tcpManager.GetDeviceByID]
        F3 --> F4{设备存在?}
        F4 -->|是| F5[Device数据返回]
        F4 -->|否| F6[404响应]
        F5 --> F7[JSON序列化响应]
        F7 --> F8[API响应返回]
    end

    %% 充电控制链路
    subgraph "充电控制链路"
        G1[充电命令API] --> G2[设备查找验证]
        G2 --> G3[PhysicalID验证]
        G3 --> G4[DNY协议构建]
        G4 --> G5[TCPWriter发送]
        G5 --> G6[设备响应处理]
        G6 --> G7[命令执行结果]
    end

    %% 数据存储层
    subgraph "数据存储层"
        H1[(ConnectionSession<br/>连接级别数据)]
        H2[(DeviceGroup<br/>设备组数据)]
        H3[(Device<br/>设备级别数据)]
        H4[(设备索引映射<br/>deviceID→iccid)]
    end

    %% 连接关系
    A6 --> C1
    C8 --> D1
    D10 --> E1
    D9 --> H1
    D9 --> H2
    D9 --> H3
    D9 --> H4

    F3 --> H3
    G2 --> H3
    E4 --> H3

    %% 样式定义
    classDef connectionFlow fill:#e1f5fe
    classDef iccidFlow fill:#f3e5f5
    classDef registerFlow fill:#e8f5e8
    classDef heartbeatFlow fill:#fff3e0
    classDef apiFlow fill:#fce4ec
    classDef commandFlow fill:#f1f8e9
    classDef dataLayer fill:#f5f5f5

    class A1,A2,A3,A4,A5,A6,B1,B2,B3,B4,B5,B6 connectionFlow
    class C1,C2,C3,C4,C5,C6,C7,C8 iccidFlow
    class D1,D2,D3,D4,D5,D6,D7,D8,D9,D10 registerFlow
    class E1,E2,E3,E4,E5,E6,E7 heartbeatFlow
    class F1,F2,F3,F4,F5,F6,F7,F8 apiFlow
    class G1,G2,G3,G4,G5,G6,G7 commandFlow
    class H1,H2,H3,H4 dataLayer
```

## 唯一发送链路与实现锚点

- 唯一链路：HTTP API → DeviceGateway → UnifiedDNYBuilder → UnifiedSender/TCPWriter → RAW TCP → 设备
- 实现文件：
  - 构包：`pkg/protocol/unified_dny_builder.go::BuildUnifiedDNYPacket`
  - 发送：`pkg/network/unified_sender.go`、`pkg/network/tcp_writer.go`
  - 消息ID：`pkg/export.go::Protocol.GetNextMessageID()`（每命令唯一；重发沿用原ID）
  - 发送节流（≥0.5s/设备）：`pkg/gateway/send.go::SendCommandToDevice`
  - 超时与重发：`pkg/network/command_manager.go`（15s、最多重发2次）

```mermaid
sequenceDiagram
  participant API as HTTP API
  participant GW as DeviceGateway
  participant B as UnifiedDNYBuilder
  participant S as UnifiedSender/TCPWriter
  participant DEV as 设备

  API->>GW: 接口请求(deviceId, cmd, data)
  GW->>GW: DeviceID标准化 + PhysicalID一致性校验
  GW->>B: BuildUnifiedDNYPacket(physicalID, msgID, cmd, data)
  B-->>GW: DNY数据帧(小端, 含校验/长度)
  GW->>S: SendDNYPacket(packet)
  S->>DEV: RAW TCP 写出
  DEV-->>S: 设备侧处理/可选应答
  S-->>GW: 写入结果
  GW-->>API: 结果响应
```

## AP3000 协议关键约束（落实于实现）

- 帧格式（小端，长度含校验）：`DNY(3)` + `Length(2)` + `PhysicalID(4)` + `MessageID(2)` + `Command(1)` + `Data(n)` + `Checksum(2)`
- 校验：自 `DNY` 起至校验字段前字节累加和的低2字节（见 `UnifiedDNYBuilder.calculateChecksum`）
- 字节序：除特别标注外均为小端；长度/物理ID/消息ID/数值字段一律小端
- 发送节流：同设备命令间隔 ≥ 0.5 秒（`DeviceGateway.SendCommandToDevice`）
- 消息ID：每命令唯一，重发保持一致；超时 15 秒，最多重发 2 次（`CommandManager`）
- 设备ID对外 API：`utils.DeviceIDProcessor.SmartConvertDeviceID` 标准化
- 统一构包/发送：仅允许 `BuildUnifiedDNYPacket` + `UnifiedSender/TCPWriter`，禁止二次封装
- 日志：下发与上行帧输出结构化字段 `deviceID/physicalID/msgID/cmd/dataHex/packetHex`

