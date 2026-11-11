# A7M5 Step 1 真机验证手册

> 状态：Draft，等待 A7M5 真机执行
> 适用阶段：Step 1 协议可行性
> 目标：证明 A7M5 能否完成 JPEG 原文件读取、单次静态拍照，并为实时取景和
> iPhone 端限制收集可复现证据

本手册不会把协议声明或虚拟相机测试结果当作 A7M5 支持证明。只有真机产物完整
并通过评审后，才能更新兼容矩阵。

## 1. 验证边界

必须先分别记录以下会话，不能假设它们可以同时使用：

1. 手机/Wi-Fi 浏览或传输模式
2. 遥控拍摄模式
3. 实时取景模式
4. iPhone 加入相机网络后的本地网络权限和生命周期行为

每次运行只改变一个条件。失败时保留证据，不通过盲试未知操作码继续。

## 2. 设备和输入

| 项目 | 要求 |
|---|---|
| 相机 | Sony A7M5 / ILCE-7M5 |
| 相机固件 | 更新到 Sony 当前最新版，并记录完整版本号 |
| 测试手机 | iPhone 14 Pro Max，iOS 18.7.7 |
| 协议主机 | Windows AMD64/ARM64 或 macOS Intel/Apple Silicon |
| 存储卡 | 可恢复或专用测试卡；禁止使用唯一副本 |
| 测试素材 | 至少一张相机生成的 JPEG 原文件 |
| 网络 | 记录相机 Wi-Fi SSID 规则、主机 IP 和是否存在其他网络 |
| 记录工具 | 截屏、文件哈希工具、本手册和 JSON 报告目录 |

开始前备份或取出不可替代的照片。测试不得包含相机内删除、格式化、固件修改或
批量设置写入。

## 3. 构建探针

从仓库根目录进入：

```powershell
cd tools/a7m5-probe
```

Windows 首次构建可直接双击 `install-go.cmd`，或执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\install-go.ps1
```

macOS：

```bash
chmod +x install-go.sh install-go.command
./install-go.sh
```

构建完成后先运行本地测试：

```powershell
go test ./...
go vet ./...
```

真机测试前确认所有测试通过。测试失败时不要连接相机。

## 4. 证据目录

每次真机轮次建立独立目录：

```text
artifacts/protocol-probes/
└── A7M5-<firmware>-<YYYYMMDD>-<run>/
    ├── environment.md
    ├── screenshots/
    ├── device-info.json
    ├── sony-handshake.json
    ├── list.json
    ├── jpeg-download.json
    ├── downloaded/
    ├── capture-once.json
    ├── events.log
    └── notes.md
```

可直接创建目录和初始环境清单：

```powershell
.\a7m5-probe.exe evidence-init --firmware <firmware> --label run1
```

`artifacts/` 默认不进入 Git。提交评审材料前必须检查其中不含 Wi-Fi 密码、相机
序列号和未脱敏的个人照片。

## 5. 测试步骤

### ENV-01 环境基线

记录：

- A7M5 固件和机身设置
- iPhone 型号、iOS 和 App 构建
- 协议主机操作系统、架构、探针构建哈希
- 相机菜单当前模式、USB/Wi-Fi/BLE 设置
- 相机电池、存储卡槽位和剩余空间
- SSID 规则和协议主机 IP；密码写入安全存储，不写入普通日志

结果：`PASS / FAIL / BLOCKED`

### NET-01 手动加入相机网络

1. 在相机上选择计划验证的模式。
2. 在协议主机系统设置中手动加入相机网络。
3. 确认协议主机的相机侧 IP。
4. 检查 TCP `15740` 是否可达。

可先运行网络诊断：

```powershell
.\a7m5-probe.exe diagnose --host <camera-ip>
```

运行只读探测：

```powershell
.\a7m5-probe.exe probe --host <camera-ip> `
  --json-out <evidence-dir>\device-info.json
```

通过条件：

- PTP/IP 初始化成功
- 读到相机型号和固件字段
- 报告中的序列号仅显示指纹
- 失败原因可读，不进行自动协议回退

### PROTO-01 Sony 能力握手

只有 NET-01 通过后执行：

```powershell
.\a7m5-probe.exe probe --host <camera-ip> --sony-handshake `
  --json-out <evidence-dir>\sony-handshake.json
```

记录协议版本、属性码、控制码和任何模式变化。操作返回成功不等于后续内容或
拍摄能力可用。

### LIST-01 内容列表

```powershell
.\a7m5-probe.exe list --host <camera-ip> `
  --json-out <evidence-dir>\list.json
```

记录：

- 卡槽数量、日期数和内容数
- 最新文件的名称、格式码和大小
- 相机 UI 是否仍可使用
- 空卡槽、无卡和相机忙碌时的错误语义

如果无法正确显示测试 JPEG，停止 JPEG 下载评审，检查相机模式而不是遍历未知
操作码。

### JPEG-01 JPEG 原文件下载

```powershell
.\a7m5-probe.exe download-jpeg --host <camera-ip> `
  --out-dir <evidence-dir>\downloaded `
  --json-out <evidence-dir>\jpeg-download.json
```

通过条件：

- 相机报告长度与下载长度完全一致
- JPEG 签名为 `FF D8 FF`
- 文件可被系统解码
- 报告中的 SHA-256 可对本地完整文件复算
- 文件扩展名、尺寸和 EXIF 与相机素材相符
- 没有把缩略图或 screennail 标成原文件

建议连续执行 3 次并比较哈希。如果相机允许重新生成同名文件，先确认远端内容
身份再比较。

### XFER-01 中断和原子落盘

在下载过程中至少测试一次断网或断开相机 Wi-Fi：

- 不得生成正式完成文件
- `.part` 文件可以保留用于诊断
- 重连后不得把 `.part` 当作完整文件提交
- JSON 和日志必须显示真实失败点

### CAP-01 单次静态拍照

只有 LIST-01 和 JPEG-01 通过后执行，并使用空白或可恢复测试卡：

```powershell
.\a7m5-probe.exe capture-once --host <camera-ip> --confirm-shutter `
  --json-out <evidence-dir>\capture-once.json
```

通过条件：

- 相机明确声明 `0x9207`、`0xD2C1` 和 `0xD2C2`
- 一次 S1/S2 序列只产生预期的一次拍摄
- 有相机事件或相机回放等独立证据确认拍摄完成
- 命令返回 OK 只记为“请求被接受”，不能单独证明快门触发
- 超时或断线后工具不自动重试
- 人工检查前不再次发送快门

### LIVE-01 实时取景

当前探针尚未实现实时取景，因此本项标记为 **TOOL PENDING**。

准备并记录以下验收口径，工具实现后再执行：

- 启动和停止步骤
- 预览分辨率
- 10 秒内收到的帧数
- 延迟测量方法和设备
- 断流、重连和锁屏行为
- 实时取景与传图是否互斥
- 相机和手机的热量、电量和降帧现象

不得用单张预览或 screennail 冒充实时取景通过。

### IOS-01 iPhone 平台边界

协议主机验证通过后，使用 iPhone 14 Pro Max / iOS 18.7.7 单独记录：

- 本地网络权限提示和拒绝后的行为
- 加入相机 Wi-Fi 的用户确认流程
- App 进入后台、锁屏和重新前台后的会话状态
- 切换网络或关闭 Wi-Fi 后的恢复
- PhotoKit 写入权限和导入结果
- 相同测试步骤与协议主机的差异

在最小 iOS App 建立前，可使用官方 App 作为行为对照，但不能把官方 App 的
结果当作第三方协议已经可用。

## 6. 失败场景

每次只触发一种故障并记录：

| 场景 | 必须记录 |
|---|---|
| TCP 不可达 | 主机路由、错误和相机模式 |
| 认证失败 | 认证阶段、相机提示，不记录密码 |
| 相机忙碌 | 原错误码、相机 UI 状态、是否可安全重试 |
| 无卡/空槽 | 原错误码和槽位，不自动改查其他模式 |
| 下载短读 | 已接收字节、`.part`、最终文件不存在 |
| 相机休眠 | 会话何时断开、重连是否恢复 |
| 快门超时 | 先检查相机，不自动重发 |
| 模式互斥 | 原模式、目标模式和任务影响 |

## 7. Go/No-Go 评审

传图 GO 条件：

- A7M5 可手动联网
- 能重复读取内容列表
- 能下载并验证 JPEG 原文件
- 失败和断线行为可解释

遥控 GO 条件：

- A7M5 明确声明并实际接受快门控制
- 一次命令仅产生一次拍摄
- 有独立完成证据
- 超时和结果未知路径不会自动重拍

实时取景 GO 条件：

- 有真实连续帧证据
- 能看到启动、停止、断流和恢复状态
- 测得的延迟/帧率口径可复现

任一核心条件不满足时暂停 Step 1 评审，不自行删除遥控目标，也不把局部成功写成
“已支持 A7M5”。

逐项证据来源和置信度见
[A7M5 协议证据矩阵](A7M5_PROTOCOL_EVIDENCE_MATRIX.md)。
