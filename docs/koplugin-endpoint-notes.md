# KOReader Z-Library 自动发现：选择性参考

调研日期：2026-10-06。只看 endpoint 发现与错误分类，基于 `ZlibraryKO/zlibrary.koplugin` 提交 [`bc2d851d`](https://github.com/ZlibraryKO/zlibrary.koplugin/tree/bc2d851d451bb45b137544312e6cdf34a4e35950)，未验证线上服务可用性。下述取舍按本仓库 EAPI First 方案确定。

## 可参考的做法

- **Probe 验证接口语义。** 插件请求 `/eapi/info/ok`，要求 2xx、非空 JSON、`success == 1`。HTTP 200 或任务执行成功都不足以证明 endpoint 可用。这里适合复用同样的判断，避免 HTML 挑战页、快速 DNS 失败被误选。[api.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/api.lua#L1844)、[discovery.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/discovery.lua#L257)
- **恢复请求尽快结束，完整扫描留给诊断。** 非交互发现收到首个真正健康的结果便停止；交互界面收集各节点耗时。本仓库普通请求可使用有界候选列表、固定顺序与一次重试，`doctor` 再展示全面检测结果。[discovery.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/discovery.lua#L264)
- **把挑战页与业务拒绝分开。** 插件有独立的 DNS、浏览器挑战、凭据拒绝、登录限流分类；挑战检测只扫描响应开头。这里应保留思路，但用 Go 类型化错误驱动策略，避免靠本地化文案匹配。[api.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/api.lua#L14)、[挑战检测](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/api.lua#L262)
- **发现源与实际可用性分离。** 候选合并后去重，再逐一 Probe；官方列表包含 `.onion`，插件会排除。本仓库还需对自动候选及重定向执行 HTTPS 与精确域名白名单校验。[config.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/config.lua#L644)、[discovery.lua](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/discovery.lua#L73)

## 不采用的部分

插件使用多个 CDN、第三方代理和官方接口轮换获取列表，交互扫描还从最快五个节点随机选择。这增加依赖与行为不确定性；首版保留内置 seeds、last-good 和官方发现即可。[发现源](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/api.lua#L1967)、[随机选择](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/discovery.lua#L301)

插件把选中地址写回 base URL 设置，且有挑战节点 TTL、多种缓存和子进程写入协调。本仓库应按设计只保存可删除的 `state.json`，保留用户配置与 session 的职责；首版无需额外持久化封禁缓存。[配置写入](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/config.lua#L705)

插件当前登录顺序是 `/rpc.php` 后 `/eapi/user/login`，注释明确允许登录拒绝后尝试另一个接口。这里只参考两种响应如何提取同一组 session 凭据；顺序与 fallback 条件仍遵循本仓库 EAPI First 方案，明确无效密码、401、429 与业务 `success=0` 不自动降级。泛化的 `Authorization failed` 不能直接视为后端不兼容。[登录接口与响应](https://github.com/ZlibraryKO/zlibrary.koplugin/blob/bc2d851d451bb45b137544312e6cdf34a4e35950/zlibrary/api.lua#L725)

## 本仓库实现

本仓库使用一个 resolver、一套错误分类和一个 last-good 文件；每次业务请求最多切换 endpoint 一次、降级 backend 一次。成功路径直接使用当前 endpoint，不做全量预扫描。`doctor` 用于排障，`endpoint` 展示模式、来源与上次成功信息。
