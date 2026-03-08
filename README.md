# Hades - LongBridge MCP Server

基于 LongBridge OpenAPI 的 HTTP JSON-RPC MCP 服务，提供港股、美股的行情、交易、提醒和定时执行能力。

## 功能

### 行情
- 实时行情: `get_quote`
- 股票基本信息: `get_quote_info`
- 盘口深度: `get_depth`
- 实时成交: `get_trades`
- K 线数据: `get_candlesticks`

### 交易
- 账户余额/购买力: `get_account_info`
- 持仓查询: `get_positions`
- 下单: `submit_order`
- 撤单: `cancel_order`
- 当日订单: `get_orders`
- 订单详情: `get_order_detail`
- 历史成交: `get_history_executions`

### 自动化
- 每日简报: `get_daily_brief`, `generate_daily_brief`
- 交易复盘: `generate_daily_review`, `generate_weekly_review`, `generate_monthly_review`, `generate_yearly_review`
- 信号提醒: `create_signal_alert`, `list_signal_alerts`, `delete_signal_alert`, `enable_signal_alert`, `check_alerts`
- 执行窗口: `create_execution_window`, `list_execution_windows`, `delete_execution_window`

### 通知
- 飞书应用消息
- Webhook: Slack / 钉钉 / 飞书

## 快速开始

### Docker

```bash
cp config.yaml.example config.yaml
cp .env.example .env

# 编辑 config.yaml 或 .env
docker-compose up -d --build
```

### 本地运行

```bash
cp config.yaml.example config.yaml

go build -o bin/server ./cmd/server
./bin/server -config config.yaml
```

默认地址:

```text
http://localhost:8080/mcp/
```

## 配置

推荐使用环境变量注入 LongBridge 凭证，不要把真实凭证提交到仓库。

```yaml
app_key: ""
app_secret: ""
access_token: ""
region: "cn"

server:
  host: "0.0.0.0"
  port: 8080

daily_brief:
  enabled: true
  timezone: "Asia/Shanghai"
  pre_market_time: "09:00"
  post_market_time: "16:00"
  webhook_url: ""

signal_alert:
  enabled: true
  check_interval: 60
  webhook_url: ""

execution_window:
  enabled: true

feishu:
  enabled: false
  app_id: ""
  app_secret: ""
  user_id: ""

log:
  level: "info"
```

环境变量:

| 变量 | 说明 |
|------|------|
| `LONGPORT_APP_KEY` | LongBridge App Key |
| `LONGPORT_APP_SECRET` | LongBridge App Secret |
| `LONGPORT_ACCESS_TOKEN` | LongBridge Access Token |
| `LONGPORT_REGION` | 地区，可选 `cn` / `hk` / `us` |
| `LONGPORT_LOG_LEVEL` | LongBridge SDK 日志级别，可选 `debug` / `info` / `warn` / `error` |

说明:
- LongBridge 底层 WebSocket 偶发 `close 1006 / unexpected EOF` 后自动重连是常见现象
- 当前服务会过滤“断开后立即重连成功”的噪声日志，真正的重连失败仍会保留

## MCP 协议说明

当前服务通过 HTTP 暴露 JSON-RPC 风格的 MCP 接口。

已实现的方法:
- `initialize`
- `ping`
- `tools/list`
- `tools/call`

## 工具清单

| 工具 | 关键参数 | 说明 |
|------|------|------|
| `get_quote` | `symbols` | 多股票实时行情，逗号分隔 |
| `get_quote_info` | `symbols` | 股票基本信息 |
| `get_depth` | `symbol` | 买卖盘深度 |
| `get_trades` | `symbol`, `start`, `end`, `count` | 实时成交，支持时间过滤 |
| `get_candlesticks` | `symbol`, `period`, `count`, `size`, `start`, `end` | K 线，支持区间查询 |
| `get_stock_news` | `symbol`, `count` | 获取个股相关新闻资讯 |
| `generate_watchlist_plan` | `symbols`, `news_count`, `lookback` | 结合周K/日K和资讯生成关注股计划 |
| `analyze_trend` | `symbol`, `periods`, `lookback` | 单标的走势分析 |
| `analyze_watchlist_trends` | `symbols`, `periods`, `lookback` | 批量走势分析 |
| `analyze_positions_trends` | `periods`, `lookback` | 当前持仓走势体检，附带浮动盈亏 |
| `analyze_portfolio_risk` | 无 | 分析当前组合集中度、弱势仓位和风险动作 |
| `generate_daily_review` | `start`, `end`, `timezone`, `periods`, `lookback` | 生成日复盘 |
| `generate_weekly_review` | `start`, `end`, `timezone`, `periods`, `lookback` | 生成周复盘 |
| `generate_monthly_review` | `start`, `end`, `timezone`, `periods`, `lookback` | 生成月复盘 |
| `generate_yearly_review` | `start`, `end`, `timezone`, `periods`, `lookback` | 生成年复盘 |
| `generate_trading_digest` | `period`, `symbols`, `news_count`, `lookback`, `timezone`, `start`, `end` | 生成交易摘要、组合风险和行动清单 |
| `get_account_info` | 无 | 账户余额与现金信息 |
| `get_positions` | 无 | 当前持仓，附带最新价和浮动盈亏 |
| `submit_order` | `symbol`, `order_type`, `side`, `quantity`, `price`, `time_in_force` | 提交订单 |
| `cancel_order` | `order_id` | 撤单 |
| `get_orders` | `status` | 当日订单，可按状态过滤 |
| `get_order_detail` | `order_id` | 订单详情 |
| `get_history_executions` | `symbol`, `start`, `end` | 历史成交 |
| `get_daily_brief` | `version`, `symbols` | 每日简报 |
| `generate_daily_brief` | `version`, `symbols` | `get_daily_brief` 别名 |
| `create_signal_alert` | `symbol`, `alert_type`, `condition`, `threshold`, `note` | 创建提醒 |
| `list_signal_alerts` | 无 | 列出提醒 |
| `delete_signal_alert` | `alert_id` | 删除提醒 |
| `enable_signal_alert` | `alert_id`, `enabled` | 启用/禁用提醒 |
| `check_alerts` | 无 | 手动触发一次提醒检查 |
| `create_execution_window` | `name`, `schedule`, `strategy`, `webhook_url`, `enabled` | 创建执行窗口 |
| `list_execution_windows` | 无 | 列出执行窗口 |
| `delete_execution_window` | `window_id` | 删除执行窗口 |

## 参数约定

### `get_trades`
- `start`, `end`: Unix 毫秒时间戳
- `count`: 返回条数，默认 `100`

### `get_candlesticks`
- `period` 支持: `1m`, `5m`, `15m`, `30m`, `1h`, `2h`, `4h`, `1d`, `1w`, `1M`
- `count` 和 `size` 等价
- `start`, `end`: Unix 毫秒时间戳
- 传 `start/end` 时优先走 LongBridge 历史 K 线区间接口

### `analyze_trend`
- `periods` 默认: `1d,1h,15m`
- `lookback` 默认: `120`
- 输出: `trend`, `score`, `signals`, `risks`, `suggestion`, `support`, `resistance`

### `get_stock_news`
- 使用 LongBridge 官方资讯接口拉取个股相关新闻
- 输出标题、摘要、链接、发布时间和互动数据

### `generate_watchlist_plan`
- 自动结合周K和日K趋势
- 输出 `buy_zone_low`、`buy_zone_high`、`stop_loss`、`take_profit`
- 附带近期资讯摘要，适合做下周关注计划

### `analyze_watchlist_trends`
- `symbols`: 多股票代码，逗号分隔
- 会返回按 `score` 降序排序的结果列表

### `analyze_positions_trends`
- 自动读取当前持仓
- 对每个持仓输出趋势评分、信号、风险、最新价和浮动盈亏
- 适合日/周复盘和飞书持仓体检

### `analyze_portfolio_risk`
- 评估单一持仓占比、前 3 大持仓集中度
- 识别弱势持仓和亏损持仓
- 输出组合级风险和动作建议

### `generate_weekly_review`
- 默认统计本周一 00:00 到当前时间
- 汇总账户快照、订单/成交统计、按股票的成交活跃度
- 会附带当前持仓的趋势体检、持仓浮动盈亏、区间已实现盈亏和周内风险提示

### `generate_daily_review`
- 默认统计当日 00:00 到当前时间
- 输出结构与周复盘一致，适合盘后日报
- 已实现盈亏基于 FIFO 和历史成交重建，当前不含手续费

### `generate_monthly_review`
- 默认统计本月 1 日 00:00 到当前时间
- 输出结构与周复盘一致，适合月度交易回顾

### `generate_yearly_review`
- 默认统计当年 1 月 1 日 00:00 到当前时间
- 输出结构与周复盘一致，适合年度回顾

### `generate_trading_digest`
- 把复盘、组合风险、关注股计划合并成一条摘要输出
- 适合 zeroClaw 直接调用，减少多次工具调用
- 会给出 `action_items` 供日报/周报直接使用
- 会额外返回 `execution_checklist`
- `execution_checklist` 包含 `buy_candidates`、`position_actions`、`risk_controls`、`priority_order`

### `get_positions`
- 返回当前持仓列表和汇总信息
- 汇总信息包含 `cost_basis`、`market_value`、`unrealized_pnl`、`unrealized_pnl_pct`
- 当前仅统计持仓浮动盈亏，已实现盈亏仍需后续成交配对计算

### `submit_order`
- `order_type` 常用值: `limit`, `market`, `LO`, `MO`, `ELO`, `AO`, `ALO`
- `side`: `buy` / `sell`
- `time_in_force`: `day` / `gtc` / `gtd`

### `get_orders`
- `status` 支持便捷值: `all`, `filled`, `cancelled`, `pending`, `failed`
- 也支持直接传 LongBridge SDK 原始状态值

### `create_signal_alert`
- `alert_type` 当前可用: `price`, `volatility`, `volume`, `trend`
- `condition` 当前可用: `above`, `below`, `cross_up`, `cross_down`, `in_buy_zone`, `near_take_profit`, `below_stop_loss`
- `trend + in_buy_zone` 可用于“到了买入区再提醒”
- `trend + near_take_profit` 可用于“接近止盈位提前提醒”
- `trend + below_stop_loss` 可用于“跌破计划止损位提醒”
- 触发后会附带当前价、关注买入区、止损位、止盈位和近期资讯摘要

### `create_execution_window`
- `schedule` 使用 cron 表达式
- 支持 5 段或 6 段格式
- 示例:
  - `0 9 * * 1-5`
  - `0 0 9 * * 1-5`

## 请求示例

### 初始化

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize"
}
```

### 获取工具列表

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

### 获取实时行情

```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "get_quote",
    "arguments": {
      "symbols": "700.HK,AAPL.US,TSLA.US"
    }
  }
}
```

### 获取区间 K 线

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "get_candlesticks",
    "arguments": {
      "symbol": "AAPL.US",
      "period": "1d",
      "start": 1704067200000,
      "end": 1706745600000,
      "count": 200
    }
  }
}
```

### 获取实时成交

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "get_trades",
    "arguments": {
      "symbol": "700.HK",
      "count": 50
    }
  }
}
```

### 分析单只股票走势

```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "analyze_trend",
    "arguments": {
      "symbol": "AAPL.US",
      "periods": "1d,1h,15m",
      "lookback": 120
    }
  }
}
```

### 批量分析 watchlist

```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "analyze_watchlist_trends",
    "arguments": {
      "symbols": "AAPL.US,TSLA.US,700.HK",
      "periods": "1d,1h",
      "lookback": 120
    }
  }
}
```

### 获取个股资讯

```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "get_stock_news",
    "arguments": {
      "symbol": "AAPL.US",
      "count": 3
    }
  }
}
```

### 生成关注股下周计划

```json
{
  "jsonrpc": "2.0",
  "id": 15,
  "method": "tools/call",
  "params": {
    "name": "generate_watchlist_plan",
    "arguments": {
      "symbols": "AAPL.US,TSLA.US,1810.HK",
      "news_count": 3,
      "lookback": 120
    }
  }
}
```

### 分析组合风险

```json
{
  "jsonrpc": "2.0",
  "id": 16,
  "method": "tools/call",
  "params": {
    "name": "analyze_portfolio_risk",
    "arguments": {}
  }
}
```

### 分析当前持仓走势

```json
{
  "jsonrpc": "2.0",
  "id": 17,
  "method": "tools/call",
  "params": {
    "name": "analyze_positions_trends",
    "arguments": {
      "periods": "1d,1h,15m",
      "lookback": 120
    }
  }
}
```

### 生成周复盘

```json
{
  "jsonrpc": "2.0",
  "id": 18,
  "method": "tools/call",
  "params": {
    "name": "generate_weekly_review",
    "arguments": {
      "timezone": "Asia/Shanghai",
      "periods": "1d,1h",
      "lookback": 120
    }
  }
}
```

### 生成交易摘要

```json
{
  "jsonrpc": "2.0",
  "id": 19,
  "method": "tools/call",
  "params": {
    "name": "generate_trading_digest",
    "arguments": {
      "period": "weekly",
      "symbols": "AAPL.US,TSLA.US,1810.HK",
      "news_count": 3,
      "lookback": 120,
      "timezone": "Asia/Shanghai"
    }
  }
}
```

### 生成月复盘

```json
{
  "jsonrpc": "2.0",
  "id": 17,
  "method": "tools/call",
  "params": {
    "name": "generate_monthly_review",
    "arguments": {
      "timezone": "Asia/Shanghai",
      "periods": "1d,1h",
      "lookback": 120
    }
  }
}
```

### 提交订单

```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "tools/call",
  "params": {
    "name": "submit_order",
    "arguments": {
      "symbol": "700.HK",
      "order_type": "limit",
      "side": "buy",
      "price": 350.0,
      "quantity": 100,
      "time_in_force": "day"
    }
  }
}
```

### 查询当日已成交订单

```json
{
  "jsonrpc": "2.0",
  "id": 21,
  "method": "tools/call",
  "params": {
    "name": "get_orders",
    "arguments": {
      "status": "filled"
    }
  }
}
```

### 获取历史成交

```json
{
  "jsonrpc": "2.0",
  "id": 22,
  "method": "tools/call",
  "params": {
    "name": "get_history_executions",
    "arguments": {
      "symbol": "AAPL.US",
      "start": 1704067200000,
      "end": 1706745600000
    }
  }
}
```

### 生成每日简报

```json
{
  "jsonrpc": "2.0",
  "id": 30,
  "method": "tools/call",
  "params": {
    "name": "generate_daily_brief",
    "arguments": {
      "version": "pre_market",
      "symbols": "700.HK,9988.HK"
    }
  }
}
```

### 创建信号提醒

```json
{
  "jsonrpc": "2.0",
  "id": 31,
  "method": "tools/call",
  "params": {
    "name": "create_signal_alert",
    "arguments": {
      "symbol": "TSLA.US",
      "alert_type": "volatility",
      "condition": "above",
      "threshold": 5.0,
      "note": "特斯拉波动剧烈"
    }
  }
}
```

### 创建买入区提醒

```json
{
  "jsonrpc": "2.0",
  "id": 32,
  "method": "tools/call",
  "params": {
    "name": "create_signal_alert",
    "arguments": {
      "symbol": "TSLA.US",
      "alert_type": "trend",
      "condition": "in_buy_zone",
      "note": "到了计划买入区再提醒我"
    }
  }
}
```

### 创建接近止盈位提醒

```json
{
  "jsonrpc": "2.0",
  "id": 33,
  "method": "tools/call",
  "params": {
    "name": "create_signal_alert",
    "arguments": {
      "symbol": "TSLA.US",
      "alert_type": "trend",
      "condition": "near_take_profit",
      "threshold": 1.5,
      "note": "距离止盈位 1.5% 以内提醒我"
    }
  }
}
```

### 创建跌破止损位提醒

```json
{
  "jsonrpc": "2.0",
  "id": 34,
  "method": "tools/call",
  "params": {
    "name": "create_signal_alert",
    "arguments": {
      "symbol": "TSLA.US",
      "alert_type": "trend",
      "condition": "below_stop_loss",
      "note": "跌破计划止损位就提醒我"
    }
  }
}
```

### 手动触发提醒检查

```json
{
  "jsonrpc": "2.0",
  "id": 35,
  "method": "tools/call",
  "params": {
    "name": "check_alerts",
    "arguments": {}
  }
}
```

### 创建执行窗口

```json
{
  "jsonrpc": "2.0",
  "id": 40,
  "method": "tools/call",
  "params": {
    "name": "create_execution_window",
    "arguments": {
      "name": "盘前持仓检查",
      "schedule": "0 9 * * 1-5",
      "strategy": "check_positions",
      "enabled": true
    }
  }
}
```

## 返回格式

`tools/call` 的返回内容放在:

```json
{
  "result": {
    "content": [
      {
        "type": "text",
        "text": "..."
      }
    ]
  }
}
```

对于结构化结果，例如 `get_candlesticks`，`text` 字段内容会是 JSON 字符串。

## 开发校验

```bash
GOCACHE=$(pwd)/.gocache go test ./...
```

## License

Mozilla Public License 2.0
