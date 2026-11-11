# A7M5 协议证据矩阵

> 更新日期：2026-11-11
> 状态：Step 1 调研基线；A7M5 真机结果全部待补
> 用途：区分官方资料、其他机型实测、静态分析和待验证假设

## 1. 结论使用规则

证据优先级如下：

1. 本项目 A7M5 真机、指定固件、指定模式和可复现产物
2. Sony 对 A7M5 或对应接口的官方说明
3. 同代其他机型的真机测试
4. 官方 App、APK 或其他实现的静态分析
5. 基于协议结构的假设

低优先级证据不能覆盖高优先级反例。例如，A7M5 真机返回
`StoreNotAvailable` 时，不能因为 A6700 支持标准 PTP 对象读取就把 A7M5
写成“已支持”。

标记：

| 标记 | 含义 |
|---|---|
| 官方 | Sony 官方页面或资料明确描述 |
| 他机实测 | 已在其他机型真机验证，只能形成 A7M5 假设 |
| 静态 | APK、公开代码或抓包分析，尚未在 A7M5 复现 |
| 假设 | 根据已有结构推导，不能用于产品承诺 |
| A7M5 待测 | 本项目的最终状态；当前没有该项真机证据 |

## 2. 证据来源

### 2.1 Sony 官方

- [Camera Remote Command](https://support.d-imaging.sony.co.jp/app/cameraremotecommand/en/index.html)
- [Camera Remote SDK](https://support.d-imaging.sony.co.jp/app/sdk/en/)
- [A7M5 手机配对](https://helpguide.sony.net/ilc/2540/v1/en/contents/202h_register_smartphone.html)
- [A7M5 遥控拍摄](https://helpguide.sony.net/di-app/ca/v1/en/contents/CA_remote_shooting.html)
- [A7M5 电脑遥控与手机连接限制](https://helpguide.sony.net/ilc/2540/v1/en/contents/0903_pc_remote_function.html)

官方当前说明 Camera Remote Command 支持 A7M5、A7C II 和 A6400，支持 USB、
有线 LAN 和无线 LAN；必须查看申请包内对应 README 的逐机型、逐接口兼容性。
“机型列入支持列表”不等于“所有 Wi-Fi 操作在手机模式下可用”。

### 2.2 其他机型或公开实现

- [sarnau/sony-camera-protocol](https://github.com/sarnau/sony-camera-protocol)：
  公开逆向协议记录，作者称在 ILCE-7RM5 / A7R V 上验证传图、快门、实时取景
  和 RAW 下载。
- [Knallli/alpha2go](https://github.com/Knallli/alpha2go)：Apache-2.0，
  在 ILCE-6700 / A6700 固件 2.00 上验证 PTP/IP、遥控、实时取景和内容传输。
- [wangrunji0408/SonyCameraKit](https://github.com/wangrunji0408/SonyCameraKit)：
  Swift/iOS 的 BLE + Wi-Fi + PTP/IP 浏览和下载实验；仓库标注为早期状态。

这些项目不能替代 A7M5 验证。A7R V（`ILCE-7RM5`）与 A7 V
（`ILCE-7M5`）是不同型号。

### 2.3 Android APK 静态线索

`SonyTransfer-v1.6.14-release.apk`：

- SHA-256：
  `4A0C2D7CE052C2241D5D12FAEB149AF46259225FF914B28D0EB9BEF09A1AA88A`
- 发现 PTP/IP、Sony SDIO、DLNA、BLE UUID 和 SSH 隧道相关实现
- 未发现 `Camera Remote Command`、README、PDF 或 SDK 的直接引用
- 未发现实际快门或实时取景控制路径

APK 可以提示协议可能存在，不能证明其品牌归属、许可证、A7M5 支持或安全性。

## 3. 连接与能力矩阵

| 能力 | 现有证据 | 置信度 | A7M5 状态 | 下一步 |
|---|---|---|---|---|
| 手动加入相机 Wi-Fi | Sony 手机连接说明 | 官方 | A7M5 待测 | NET-01 |
| TCP `15740` PTP/IP | 官方 PTP Command 描述；A7R V/A6700 实测；APK | 官方 + 他机实测 | A7M5 待测 | NET-01 / PROTO-01 |
| PTP/IP command/event 双通道 | A7R V/A6700 实测；APK | 他机实测 | A7M5 待测 | PROTO-01 |
| `GetDeviceInfo` 读取型号/固件 | A6700 实测；标准 PTP | 他机实测 | A7M5 待测 | PROTO-01 |
| Sony `0x9201` SDIO Connect | A6700 实测；APK | 他机实测 | A7M5 待测 | PROTO-01 |
| Sony `0x9202` 扩展能力表 | A6700 实测 | 他机实测 | A7M5 待测 | PROTO-01 |
| Access Authentication / SSH `22` | 官方相机设置路径；A6700 实测；APK | 官方 + 他机实测 | A7M5 待测 | 单独认证测试 |
| 标准 PTP Storage/Object 读取 | 标准协议存在；A6700/A7R V 某些模式不提供 | 假设 | A7M5 待测 | LIST-01 |
| `0x923B` 日期列表 | A6700 实测；APK | 他机实测 | A7M5 待测 | LIST-01 |
| `0x923C` 内容列表 | A6700 实测；APK | 他机实测 | A7M5 待测 | LIST-01 |
| `0x923D` 内容分片读取 | A6700/A7R V 实测；APK | 他机实测 | A7M5 待测 | JPEG-01 |
| `0x923E` 缩略图/预览数据 | A6700 实测；APK | 他机实测 | A7M5 待测 | STEP 3 |
| JPEG 原文件下载 | A7R V/A6700 实测 | 他机实测 | A7M5 待测 | JPEG-01 |
| HEIF 原文件下载 | A7R V/A6700 部分实现；A7M5 官方支持格式 | 官方 + 他机实测 | A7M5 待测 | STEP 3 |
| RAW/ARW 原文件下载 | A7R V/A6700 实测路径；A7M5 官方支持 RAW | 官方 + 他机实测 | A7M5 待测 | STEP 3 |

## 4. 遥控与实时取景矩阵

| 能力 | 现有证据 | 置信度 | A7M5 状态 | 下一步 |
|---|---|---|---|---|
| `0x9207 SDIO_ControlDevice` | A7R V/A6700 实测；APK 能力探测 | 他机实测 | A7M5 待测 | CAP-01 |
| `0xD2C1` 半按 S1 | A7R V/A6700 实测 | 他机实测 | A7M5 待测 | CAP-01 |
| `0xD2C2` 全按 S2 | A7R V/A6700 实测 | 他机实测 | A7M5 待测 | CAP-01 |
| 快门命令 ACK 等于拍摄成功 | 已明确否定；ACK 只表示接受 | 他机实测反例 | 不采用 | CAP-01 |
| `0xC206` CapturedEvent 作为完成信号 | A6700 事件表；A7R V 行为依赖模式 | 他机实测 | A7M5 待测 | CAP-01 |
| 快门超时后自动重试 | 会重复拍摄，禁止 | 安全约束 | 不采用 | CAP-01 |
| 实时取景 URL 属性 `0xD278` | A6700/A7R V 实测 | 他机实测 | A7M5 待测 | LIVE-01 |
| 实时取景通过 HTTP `60152` | A6700/A7R V 实测 | 他机实测 | A7M5 待测 | LIVE-01 |
| PTP `GetObject` 获取实时取景 | A7R V 验证失败 | 他机实测反例 | 不采用 | LIVE-01 |
| 实时取景连续帧、延迟和恢复 | A7R V/A6700 部分记录 | 他机实测 | A7M5 待测 | LIVE-01 |
| 遥控和内容传输可同时使用 | 因机型和模式而异 | 假设 | A7M5 待测 | 模式互斥测试 |
| 传输模式锁定相机 UI | A6700 实测；APK 有类似分支 | 他机实测 | A7M5 待测 | LIST-01 |

## 5. iPhone 平台矩阵

| 能力 | 现有证据 | 置信度 | 状态 | 下一步 |
|---|---|---|---|---|
| 第三方 App 读取/显示相机 SSID | iOS 平台限制；需要使用系统允许的接口 | 官方平台约束 | 待实现验证 | IOS-01 |
| 自动加入相机 Wi-Fi | 需要 Hotspot Configuration entitlement 和用户确认 | 官方平台约束 | 未验证 | IOS-01 |
| 本地网络访问权限 | iOS 本地网络隐私要求 | 官方平台约束 | 未验证 | IOS-01 |
| iOS 普通 App 通用 Wi-Fi 扫描 | 不提供通用扫描接口 | 官方平台约束 | 不承诺 | IOS-01 |
| 后台持续 PTP/IP Socket | 非 Background URLSession，不能保证持续 | 官方平台约束 | 不承诺 | IOS-01 |
| PhotoKit 原文件导入 | iOS 系统能力存在 | 官方平台约束 | 未验证 | STEP 2 |

## 6. 已知冲突与高风险点

1. **官方支持列表不等于命令矩阵**：必须查看 A7M5 对应 Command README。
2. **标准 PTP 存在不等于无线模式暴露对象读取**：A6700/A7R V 已出现
   `StoreNotAvailable` 或模式差异。
3. **ACK 不等于快门成功**：必须用事件或相机侧结果确认。
4. **实时取景可能不是 PTP 对象**：公开实现记录为 HTTP 流。
5. **内容传输可能锁定相机 UI**：先读取能力，再决定是否启用传输模式。
6. **A7M5 与 A7R V 不能互相替代**：只可共享测试假设。
7. **APK 没有直接引用 Sony 文档的证据**：只能作为静态线索。
8. **第三方文档镜像不是许可证**：不得下载进仓库或再分发。

## 7. A7M5 评审输出

每项最终只能填写以下之一：

- `PASS`：A7M5 真机复现，证据完整
- `FAIL`：A7M5 真机明确不支持
- `MODEL/MODE LIMITED`：只在记录的型号、固件和模式下支持
- `UNVERIFIED`：没有 A7M5 证据
- `NOT TESTED`：尚未设计测试

执行步骤、证据目录和 Go/No-Go 口径见
[A7M5 Step 1 真机验证手册](A7M5_STEP1_RUNBOOK.md)。
