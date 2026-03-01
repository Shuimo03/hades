# PRD: 智能交易助手 - 自动化提醒与执行模块

## 1. 概述

### 1.1 背景

当前 MCP Server 已实现基础的行情查询、账户管理、订单交易功能。随着使用深入，用户需要：
- 每日自动获取持仓总结与风险提示
- 市场机会与风险实时监控
- 定时策略执行的提醒机制

### 1.2 目标

在现有 MCP Server 基础上，新增三大自动化模块：
1. **Daily Brief** - 每日开盘前/收盘后自动生成报告
2. **Signal Alert** - 市场条件触发式自动提醒
3. **Execution Window** - 定时策略执行提醒

### 1.3 范围

- **运行环境**: 集成到现有 MCP Server
- **目标市场**: 港股 + 美股（基于 LongBridge API）
- **提醒方式**: MCP 工具响应 + 即时通讯（Slack/钉钉 Webhook）

---

## 2. 功能需求

### 2.1 Daily Brief（日更报告）

#### 2.1.1 功能描述

每日定时生成交易报告，分开盘前与收盘后两个版本。

#### 2.1.2 报告内容

| 版本 | 模块 | 内容 |
|------|------|------|
| **开盘前** | 宏观摘要 | 主要股指期货走势（可选）、重要财经日历事件 |
| | 持仓概览 | 持仓股票列表、市值、盈亏情况 |
| | 风险提示 | 持仓集中度风险、单日最大潜在亏损 |
| **收盘后** | 宏观摘要 | 隔夜美股/港股市场涨跌 |
| | 持仓概览 | 今日持仓变动、盈亏统计 |
| | 风险提示 | 触发止盈/止损的持仓、流动性风险 |

#### 2.1.3 触发时间

| 报告 | 默认时间 | 可配置 |
|------|----------|--------|
| 开盘前 Brief | A股: 09:00 / 港股: 09:15 / 美股: 09:00 | 是 |
| 收盘后 Brief | A股: 15:30 / 港股: 16:15 / 美股: 16:00 | 是 |

#### 2.1.4 输出格式

```
📊 Daily Brief - [开盘前/收盘后] [日期]

=== 宏观概览 ===
[宏观数据内容]

=== 持仓概览 ===
[持仓列表与统计]

=== 风险提示 ===
[风险提醒内容]
```

#### 2.1.5 工具接口

```yaml
工具名: get_daily_brief
参数:
  - version: string (可选, "pre_market" | "post_market", 默认自动判断)
  - symbols: string[] (可选, 指定关注的股票)

工具名: set_daily_brief_time
参数:
  - pre_market_time: string (可选, 如 "09:00")
  - post_market_time: string (可选, 如 "16:00")
  - enabled: boolean (可选, 默认 true)
  - webhook_url: string (可选, Slack/钉钉 Webhook)
```

---

### 2.2 Signal Alert（信号提醒）

#### 2.2.1 功能描述

当市场满足预设条件时自动触发提醒。

#### 2.2.2 信号类型

| 信号类型 | 触发条件 | 提醒内容 |
|----------|----------|----------|
| **价格提醒** | 股价触及预设价位（止盈/止损） | "🔔 [股票] 达到 [价格]，触发 [止盈/止损]" |
| **波动率提醒** | 日内波动超过 X% | "⚡ [股票] 波动率飙升 [X]%，当前价格 [价格]" |
| **趋势破位** | 价格跌破/突破均线 | "📉 [股票] 跌破 [MA5]，趋势转弱" |
| **财报提醒** | 距离财报发布 N 天 | "📅 [股票] 将在 [N] 天后发布财报" |
| **成交量异常** | 成交量放大至 N 倍 | "📊 [股票] 成交量异常放大 [N] 倍" |

#### 2.2.3 信号配置

每个信号需配置：
- `symbol`: 股票代码
- `condition`: 触发条件
- `threshold`: 阈值
- `enabled`: 是否启用

#### 2.2.4 工具接口

```yaml
工具名: create_signal_alert
参数:
  - symbol: string (必填, 如 "700.HK" 或 "AAPL")
  - alert_type: string (必填, "price" | "volatility" | "trend" | "earnings" | "volume")
  - condition: string (必填, "above" | "below" | "cross_up" | "cross_down")
  - threshold: number (必填, 触发阈值)
  - note: string (可选, 备注)

工具名: list_signal_alerts
参数: (无)

工具名: delete_signal_alert
参数:
  - alert_id: string (必填)

工具名: enable_signal_alert
参数:
  - alert_id: string (必填)
  - enabled: boolean (必填)
```

#### 2.2.5 实现说明

- 信号监控通过定时轮询实现（默认每 60 秒检查一次）
- 价格数据从 LongBridge RealtimeQuote 获取
- 波动率/均线基于实时数据计算
- 财报日期需预先配置或通过 API 获取

---

### 2.3 Execution Window（执行窗口）

#### 2.3.1 功能描述

在预设时间点提醒用户执行策略（非自动下单）。

#### 2.3.2 执行窗口类型

| 窗口类型 | 描述 | 示例 |
|----------|------|------|
| **每日定时** | 每天固定时间 | 每天 09:00 确认持仓 |
| **每周定时** | 每周特定日期时间 | 每周一 09:30 检查建仓 |
| **定期复盘** | 月末/季末复盘 | 每月最后一个交易日复盘 |

#### 2.3.3 提醒内容

```
⏰ Execution Window - [窗口名称]

时间: [触发时间]
类型: [窗口类型]

=== 待执行策略 ===
[用户预设的策略描述]

=== 当前持仓状态 ===
[持仓概览]

=== 快速操作 ===
[预设的快捷操作按钮/指令]
```

#### 2.3.4 工具接口

```yaml
工具名: create_execution_window
参数:
  - name: string (必填, 窗口名称)
  - schedule: string (必填, cron 表达式或预设值如 "0 9 * * 1-5")
  - strategy: string (必填, 策略描述)
  - webhook_url: string (可选)
  - enabled: boolean (可选, 默认 true)

工具名: list_execution_windows
参数: (无)

工具名: update_execution_window
参数:
  - window_id: string (必填)
  - name: string (可选)
  - schedule: string (可选)
  - strategy: string (可选)
  - enabled: boolean (可选)

工具名: delete_execution_window
参数:
  - window_id: string (必填)
```

---

## 3. 配置文件

### 3.1 新增配置项

```yaml
# config.yaml

# Daily Brief 配置
daily_brief:
  enabled: true
  pre_market_time: "09:00"      # 开盘前报告时间
  post_market_time: "16:00"     # 收盘后报告时间
  timezone: "Asia/Shanghai"     # 时区
  webhook_url: ""               # Slack/钉钉 Webhook

# Signal Alert 配置
signal_alert:
  enabled: true
  check_interval: 60             # 检查间隔（秒）
  alerts:                       # 预设提醒
    - symbol: "700.HK"
      alert_type: "price"
      condition: "below"
      threshold: 350.0
      enabled: true

# Execution Window 配置
execution_window:
  enabled: true
  windows:
    - name: "早盘确认"
      schedule: "0 9 * * 1-5"
      strategy: "确认持仓状态，检查是否需要调仓"
```

---

## 4. 技术设计

### 4.1 模块架构

```
internal/
├── scheduler/           # 定时任务调度器
│   ├── scheduler.go    # 基于 cron 的定时调度
│   └── job.go          # 任务定义
├── alerts/             # 告警模块
│   ├── manager.go      # 告警管理器
│   ├── signal.go       # 信号检测
│   └── notifier.go     # 通知发送 (MCP + Webhook)
├── brief/              # 日更报告模块
│   └── generator.go    # 报告生成器
└── config/             # 配置扩展
```

### 4.2 MCP 工具注册

```go
// cmd/server/main.go 新增
tools.RegisterTool("get_daily_brief", tools.NewDailyBriefTool(lb, cfg))
tools.RegisterTool("set_daily_brief_time", tools.NewSetBriefTimeTool(scheduler, cfg))
tools.RegisterTool("create_signal_alert", tools.NewCreateSignalAlertTool(alertMgr))
tools.RegisterTool("list_signal_alerts", tools.NewListSignalAlertsTool(alertMgr))
tools.RegisterTool("delete_signal_alert", tools.NewDeleteSignalAlertTool(alertMgr))
tools.RegisterTool("create_execution_window", tools.NewCreateExecutionWindowTool(scheduler))
tools.RegisterTool("list_execution_windows", tools.NewListExecutionWindowsTool(scheduler))
tools.RegisterTool("delete_execution_window", tools.NewDeleteExecutionWindowTool(scheduler))
```

### 4.3 数据存储

- **告警配置**: 持久化到 JSON 文件 (`data/alerts.json`)
- **执行窗口配置**: 持久化到 JSON 文件 (`data/windows.json`)
- ** Brief 发送记录**: 持久化到 JSON 文件 (`data/brief_history.json`)

---

## 5. 非功能需求

### 5.1 性能

- 信号检查延迟 < 5 秒
- Daily Brief 生成时间 < 3 秒

### 5.2 可用性

- Webhook 发送失败时降级到 MCP 响应
- 配置变更无需重启服务

### 5.3 安全

- Webhook URL 支持环境变量引用
- 敏感配置（API Key）不记录日志

---

## 6. 里程碑

| 阶段 | 功能 | 预估工作量 |
|------|------|------------|
| Phase 1 | Scheduler 基础框架 + Daily Brief | 1-2 天 |
| Phase 2 | Signal Alert 核心逻辑 | 2-3 天 |
| Phase 3 | Execution Window | 1 天 |
| Phase 4 | Webhook 集成 + 优化 | 1 天 |
| **总计** | | **5-7 天** |

---

## 7. 附录

### 7.1 Cron 表达式示例

| 表达式 | 含义 |
|--------|------|
| `0 9 * * 1-5` | 每天 9:00（工作日） |
| `0 16 * * 1-5` | 每天 16:00（工作日） |
| `0 9 * * 1` | 每周一 9:00 |
| `0 0 1 * *` | 每月 1 日 00:00 |

### 7.2 Webhook 消息格式

**Slack:**
```json
{
  "text": "📊 Daily Brief - 开盘前",
  "blocks": [...]
}
```

**钉钉:**
```json
{
  "msgtype": "markdown",
  "markdown": {
    "title": "Daily Brief",
    "text": "..."
  }
}
```
