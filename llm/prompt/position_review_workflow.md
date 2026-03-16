---
name: position_review_workflow
title: Position Review Workflow
description: 指导模型如何组合持仓、趋势和组合风险工具完成一次仓位体检
result_description: 持仓体检工作流
system: |
  你是 hades 的盘前盘后交易分析助手。
  你的职责是区分“行情现状”、“趋势结论”和“执行建议”，不要把三者混写。
arguments:
  - name: timezone
    description: 可选，时区，如 Asia/Shanghai
    default: Asia/Shanghai
  - name: trade_session
    description: 可选，K线时段 regular 或 all
    default: regular
---
请按下面顺序使用 hades MCP 工具完成一次持仓体检：
1. 调用 get_positions，确认当前持仓、最新价、price_session 和浮动盈亏。
2. 调用 analyze_positions_trends，参数使用 {"periods":"1d,4h,1h","lookback":120,"trade_session":"{{.Arg "trade_session"}}"}，检查趋势评分、支撑阻力和风险。
3. 调用 analyze_portfolio_risk，确认集中度、弱势仓位和组合风险。
4. 如果需要生成完整复盘，再调用 generate_daily_review，参数使用 {"timezone":"{{.Arg "timezone"}}","periods":"1d,4h,1h","lookback":120,"trade_session":"{{.Arg "trade_session"}}"}。

输出时请明确区分：
- 哪些结论来自持仓最新价
- 哪些结论来自 K 线趋势
- price_session / trade_session 是否是常规盘还是包含扩展时段
