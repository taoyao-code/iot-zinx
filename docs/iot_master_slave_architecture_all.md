# 总体架构图

```mermaid
graph TD
    subgraph 服务器
        S["服务器\n(管理所有设备)"]
        TCPServer["TCP服务器\n(端口7054)"]
        HTTPServer["HTTP API服务器\n(端口7055)"]
        DeviceMonitor["设备监控器"]
        CommandManager["命令管理器"]
        RedisDB["Redis数据库"]
    end
    subgraph 主机
        M["主机\n(有SIM卡)"]
        SIM["SIM卡\n(ICCID)"]
        Comm["通信模块\n(2G/4G/NB/WIFI)"]
        RTC["RTC模块\n(可选)"]
        M -- "内置" --> SIM
        M -- "内置" --> Comm
        M -- "可选" --> RTC
    end
    subgraph 分机群
        F1["分机1\n(无SIM卡)"]
        F2["分机2\n(无SIM卡)"]
        F3["分机N\n(无SIM卡)"]
        F1 -- "串联" --> F2
        F2 -- "串联" --> F3
    end
    S -- "包含" --> TCPServer
    S -- "包含" --> HTTPServer
    S -- "包含" --> DeviceMonitor
    S -- "包含" --> CommandManager
    S -- "使用" --> RedisDB
    TCPServer <--> |"TCP长连接"| Comm
    M <--> |"485/LORA等组网"| F1
    F1 <--> F2
    F2 <--> F3
    %% 说明：主机通过SIM卡与服务器通信，分机通过串联方式与主机通信
```

# DNY协议数据框架

```mermaid
flowchart LR
    subgraph 数据包结构
        A["包头\n(DNY)\n3字节"] --> B["长度\n2字节"]
        B --> C["物理ID\n4字节"]
        C --> D["消息ID\n2字节"]
        D --> E["命令\n1字节"]
        E --> F["数据\nn字节"]
        F --> G["校验\n2字节"]
    end
    %% 说明：DNY协议的基本数据结构，适用于所有设备与服务器间通信
```

# 设备类型分类图

```mermaid
graph TD
    Device["设备类型"]
    Device --> SinglePort["单路插座\n(识别码03)"]
    Device --> DualPort["双路插座\n(识别码04)"]
    Device --> TenPort["10路充电桩\n(识别码05)"]
    Device --> SixteenPort["16路充电桩\n(识别码06)"]
    Device --> TwelvePort["12路充电桩\n(识别码07)"]
    Device --> Master["主机\n(识别码09)"]
    Device --> LeakageMaster["漏保主机\n(识别码0A)"]
    
    SinglePort --> OldSingle["旧款485单路"]
    SinglePort --> NewSingle["新款485单路"]
    
    DualPort --> OldDual["旧款485双路"]
    DualPort --> NewDual["新款485双路"]
    DualPort --> NBDual["新款NB双模"]
    DualPort --> WIFIDual["新款WIFI双模"]
    DualPort --> LoraDual["新款Lora双模"]
    
    TenPort --> OldTen["旧款10路"]
    TenPort --> NewTen["新款10路"]
    
    SixteenPort --> OldSixteen["旧款16路"]
    SixteenPort --> NewSixteen["新款16路"]
    
    Master --> OldMaster["旧款主机"]
    Master --> LoraMaster["LORA主机"]
    Master --> WirelessMaster["无线主机"]
    
    LeakageMaster --> StandardLeakage["标准漏保主机"]
    LeakageMaster --> AdvancedLeakage["高级漏保主机"]
    %% 说明：设备类型与识别码的对应关系
```

# 数据流图

```mermaid
flowchart TD
    S["服务器"]
    M["主机"]
    F["分机"]
    SIM["SIM卡"]
    
    subgraph 连接建立流程
        C1["建立TCP连接"]
        C2["发送SIM卡号(ICCID)"]
        C3["发送心跳维持连接(link)"]
        C1 --> C2
        C2 --> C3
    end
    
    subgraph 设备注册流程
        R1["发送注册包(20指令)"]
        R2["获取服务器时间(22指令)"]
        R3["开始定时心跳"]
        R1 --> R2
        R2 --> R3
    end
    
    subgraph 心跳维持流程
        H1["发送心跳包(21指令/01指令)"]
        H2["服务器应答"]
        H3["等待下一个心跳周期"]
        H1 --> H2
        H2 --> H3
        H3 --> H1
    end
    
    S <--> |"注册包、心跳包、指令、应答、升级等"| M
    M <--> |"轮询(00指令)、状态、指令、应答等"| F
    M -- "内置" --> SIM
    %% 说明：主机与服务器之间通过SIM卡进行TCP通信，主机与分机之间通过串口通信
```

# 状态机图

```mermaid
stateDiagram-v2
    [*] --> 上电
    上电 --> 连接服务器: 通过SIM卡
    连接服务器 --> 发送ICCID: 建立连接
    发送ICCID --> 注册中: 发送注册包(20指令)
    注册中 --> 获取时间: 发送22指令
    获取时间 --> 在线: 注册成功
    在线 --> 心跳: 定时发送心跳包(21/01指令)
    心跳 --> 在线: 心跳应答正常
    在线 --> 充电中: 接收到充电指令(82指令)
    充电中 --> 充电完成: 达到充电条件
    充电完成 --> 在线: 发送结算包(03指令)
    在线 --> 升级中: 接收到升级指令(E0/F8指令)
    升级中 --> 在线: 升级完成
    在线 --> 异常: 通信异常/故障
    异常 --> 离线: 长时间无应答
    离线 --> 上电: 重新上电
    %% 说明：设备状态流转过程，包括注册、心跳、充电、升级等关键环节
```

# 实体关系图

```mermaid
erDiagram
    USER ||--o{ ORDER : 下单
    USER ||--o{ DEVICE : 绑定
    DEVICE ||--o{ PORT : 包含
    DEVICE ||--o{ ORDER : 产生
    DEVICE ||--o{ COMMAND : 接收
    DEVICE }o--|| SIMCARD : 内置
    DEVICE }|--o{ DEVICE : 主从关系
    ORDER ||--o{ COMMAND : 包含
    COMMAND }o--|| INSTRUCTION : 类型
    PORT ||--o{ ORDER : 关联
    %% 说明：用户可绑定多个设备和订单，设备可接收多个指令，主机内置SIM卡，主机和分机形成主从关系
```

# 服务器组件架构图

```mermaid
flowchart TD
    subgraph 网关服务器架构
        TCPServer["TCP服务器\n(Zinx框架)"]
        HTTPServer["HTTP API服务器"]
        DeviceMonitor["设备监控器"]
        CommandManager["命令管理器"]
        DeviceManager["设备管理器"]
        DeviceGroupManager["设备组管理器"]
        SessionManager["设备会话管理器"]
        HeartbeatManager["心跳管理器"]
        RedisClient["Redis客户端"]
        
        TCPServer --> |"接收连接"| SessionManager
        SessionManager --> |"管理设备会话"| DeviceManager
        DeviceManager --> |"组织设备组"| DeviceGroupManager
        DeviceMonitor --> |"监控设备状态"| DeviceManager
        CommandManager --> |"管理设备命令"| DeviceManager
        HeartbeatManager --> |"发送心跳"| DeviceManager
        HTTPServer --> |"提供API"| DeviceManager
        DeviceManager --> |"存储设备数据"| RedisClient
    end
    %% 说明：服务器组件架构，展示各组件之间的关系
```

# 设备通信分析图

```mermaid
graph TD
    subgraph 通信特性
        Protocol["DNY协议"]
        Protocol --> |"包头"| Header["DNY (0x44 0x4E 0x59)"]
        Protocol --> |"物理ID"| PhysicalID["4字节设备标识"]
        Protocol --> |"消息ID"| MessageID["2字节消息序号"]
        Protocol --> |"命令"| Command["1字节命令类型"]
        Protocol --> |"数据"| Data["n字节命令数据"]
        Protocol --> |"校验"| Checksum["2字节累加和校验"]
        
        PhysicalID --> |"结构"| IDStructure["1字节设备识别码 + 3字节设备编号"]
        
        Command --> |"设备→服务器"| UpCommand["上行命令\n01:旧心跳\n20:注册\n21:新心跳\n22:获取时间\n02:刷卡\n03:结算\n06:充电心跳"]
        Command --> |"服务器→设备"| DownCommand["下行命令\n81:查询状态\n82:充电控制\n83-8F:参数设置\nE0/F8:固件升级"]
    end
    %% 说明：DNY协议的组成和主要命令类型
```

# 泳道图

```mermaid
%% Mermaid泳道图
flowchart TD
    subgraph 服务器
        S1["等待设备连接"]
        S2["接收ICCID"]
        S3["处理注册包(20指令)"]
        S4["处理心跳包(21/01指令)"]
        S5["下发充电/升级指令"]
        S6["接收结算/状态"]
    end
    subgraph 主机
        M1["上电/连接服务器"]
        M2["发送ICCID"]
        M3["发送注册包(20指令)"]
        M4["获取服务器时间(22指令)"]
        M5["定时心跳(21/01指令)"]
        M6["接收服务器指令"]
        M7["轮询分机(00指令)"]
        M8["上报状态/结算"]
    end
    subgraph 分机
        F1["上电等待主机轮询"]
        F2["响应主机轮询"]
        F3["执行充电指令"]
        F4["上报状态/结算"]
    end
    S1 --> S2
    S2 --> S3
    S3 --> S4
    M1 --> M2
    M2 --> M3
    M3 --> M4
    M4 --> M5
    S4 -- "应答" --> M5
    S5 -- "充电/升级指令" --> M6
    M6 -- "转发指令" --> M7
    M7 -- "轮询/指令" --> F1
    F1 -- "响应/状态" --> F2
    F2 -- "状态上报" --> M8
    M8 -- "状态/结算" --> S6
    F3 -- "执行结果" --> F4
    F4 -- "状态/结算" --> M8
    %% 说明：主机、分机、服务器三方协作流程
```

# 注册流程图

```mermaid
flowchart TD
    上电 --> 连接服务器
    连接服务器 --> 发送ICCID
    发送ICCID --> 发送注册包
    发送注册包 --> 等待应答
    等待应答 -- "成功" --> 获取服务器时间
    等待应答 -- "超时/失败" --> 重试注册
    重试注册 --> 等待应答
    获取服务器时间 --> 注册完成
    注册完成 --> 进入心跳流程
    %% 说明：设备注册流程，包括ICCID发送和注册包处理
```

# 心跳流程图

```mermaid
flowchart TD
    定时触发 --> 发送心跳包
    发送心跳包 --> 等待服务器应答
    等待服务器应答 -- "收到应答" --> 更新心跳时间
    等待服务器应答 -- "未收到应答" --> 重发心跳包
    重发心跳包 --> 等待服务器应答
    更新心跳时间 --> 等待下次心跳
    等待下次心跳 --> 定时触发
    %% 说明：设备心跳机制，保证设备在线状态监控
```

# 充电流程图

```mermaid
flowchart TD
    用户操作 --> 服务器下发充电指令
    服务器下发充电指令 --> |"82指令"| 主机接收指令
    主机接收指令 --> 主机轮询分机
    主机轮询分机 --> |"00指令"| 分机执行充电
    分机执行充电 --> 分机发送充电心跳
    分机发送充电心跳 --> |"06指令"| 主机汇总
    主机汇总 --> |"06指令"| 服务器上报
    分机充电完成 --> 分机发送结算
    分机发送结算 --> |"03指令"| 主机汇总
    主机汇总 --> |"03指令"| 服务器上报
    %% 说明：充电流程涉及服务器、主机、分机多方协作
```

# 升级流程图

```mermaid
flowchart TD
    服务器检测新固件 --> 服务器下发升级指令
    服务器下发升级指令 --> |"E0/F8指令"| 主机接收升级指令
    主机接收升级指令 --> |"主机升级"| 主机应答升级请求
    主机应答升级请求 --> 服务器发送固件包
    服务器发送固件包 --> 主机接收固件包
    主机接收固件包 --> |"校验"| 主机应答固件包
    主机应答固件包 -- "成功" --> 服务器发送下一包
    主机应答固件包 -- "失败" --> 服务器重发当前包
    服务器发送下一包 --> 主机接收固件包
    主机升级完成 --> 主机重启
    主机重启 --> 设备重新注册
    %% 说明：升级流程需保证数据完整性和重试机制
```

# 充电时序图

```mermaid
sequenceDiagram
    participant U as 用户
    participant S as 服务器
    participant M as 主机
    participant F as 分机
    U->>S: 充电请求
    S->>M: 下发充电指令（82指令）
    M->>F: 轮询/下发充电指令（00指令）
    F-->>M: 执行充电
    F->>M: 充电状态心跳（06指令）
    M->>S: 转发充电状态心跳（06指令）
    F->>M: 充电完成结算（03指令）
    M->>S: 转发结算信息（03指令）
    S-->>U: 充电完成通知
    %% 说明：充电过程涉及多方消息交互，需保证一致性
```

# 升级时序图

```mermaid
sequenceDiagram
    participant S as 服务器
    participant M as 主机
    participant F as 分机
    S->>M: 下发升级指令（E0/F8指令）
    M-->>S: 应答升级请求
    S->>M: 发送固件包1
    M-->>S: 应答包1成功
    S->>M: 发送固件包2
    M-->>S: 应答包2成功
    S->>M: 发送固件包n
    M-->>S: 应答包n成功
    M->>M: 升级完成/重启
    M->>S: 重新注册（20指令）
    %% 说明：升级过程需保证包完整性和重试机制
```

# 主要命令数据流图

```mermaid
flowchart TD
    subgraph 上行命令
        UP1["01:旧版心跳包"]
        UP2["20:设备注册包"]
        UP3["21:新版心跳包"]
        UP4["22:获取服务器时间"]
        UP5["02:刷卡操作"]
        UP6["03:结算消费信息"]
        UP7["06:充电功率心跳"]
    end
    
    subgraph 下行命令
        DOWN1["81:查询设备状态"]
        DOWN2["82:充电控制"]
        DOWN3["83-85:运行参数设置"]
        DOWN4["86:用户卡参数设置"]
        DOWN5["87:复位重启设备"]
        DOWN6["88:存储器清零"]
        DOWN7["E0/F8:固件升级"]
    end
    
    设备 -- "上行命令" --> 服务器
    服务器 -- "下行命令" --> 设备
    %% 说明：主要指令在设备和服务器之间流转
```

# 网关内部组件通信图

```mermaid
sequenceDiagram
    participant TCP as TCP服务器
    participant Session as 会话管理器
    participant Device as 设备管理器
    participant Group as 设备组管理器
    participant Monitor as 设备监控器
    participant Command as 命令管理器
    participant Redis as Redis数据库
    
    TCP->>Session: 新连接建立
    Session->>Device: 创建设备会话
    Device->>Group: 添加到设备组
    Device->>Monitor: 注册监控
    TCP->>Session: 接收数据
    Session->>Device: 处理设备数据
    Device->>Command: 创建命令
    Command->>Device: 处理命令结果
    Device->>Redis: 保存设备状态
    Monitor->>Device: 检查设备心跳
    Monitor->>Session: 关闭超时连接
    %% 说明：网关内部组件之间的通信流程
```
