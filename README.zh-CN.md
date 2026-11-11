**语言 / Language:** [English](README.md) | [简体中文](README.zh-CN.md)

# SonyFastConnect

SonyFastConnect 是一个以本地优先为原则的 Sony 相机照片传输与遥控可行性项目，
首阶段目标是在 iOS 上支持 A7M5（ILCE-7M5）。

## 当前状态

- Step 0 产品范围已确认。
- Step 1 A7M5 协议可行性验证正在进行。
- 当前没有将任何 Sony 相机型号标记为“已支持”。
- JPEG 原文件下载、单次静态拍照和实时取景仍需 A7M5 真机证据。

目标验证环境：

- iPhone 14 Pro Max，iOS 18.7.7
- Sony A7M5 / ILCE-7M5，相机固件将在真机测试时记录

## 文档

- [产品需求与系统设计](REQUIREMENTS_AND_DESIGN.md)
- [A7M5 协议探针](tools/a7m5-probe/README.md)

## 协议探针

当前第一个开发产物是位于 `tools/a7m5-probe` 的命令行实验工具，可在
Windows 或 macOS 上运行，覆盖：

- PTP/IP 连接与设备信息读取
- Sony SDIO 能力协商
- 相机内容列表读取
- JPEG 原文件下载，并校验字节数、JPEG 文件签名和 SHA-256
- 一次需要显式确认的快门序列，失败后不自动重试

该探针仅用于真机可行性实验，不属于未来的 iOS App 实现，也不代表已经决定
正式产品使用 Swift 还是 Rust 作为协议内核。

## 许可证

Apache License 2.0。参见 [LICENSE](LICENSE) 和
[第三方声明](tools/a7m5-probe/THIRD_PARTY_NOTICES.md)。
