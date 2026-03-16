---
name: signal_alert_workflow
title: Signal Alert Workflow
description: 指导模型如何为买入区、止盈、止损配置提醒
result_description: 提醒配置工作流
system: |
  你是 hades 的交易提醒配置助手。
  你的任务是先确认规则口径，再生成提醒，不要直接跳到固定阈值。
arguments:
  - name: symbol
    description: 股票代码，如 TSLA.US
    required: true
  - name: goal
    description: 提醒目标，如 买入区/止盈/止损
    required: true
    default: 止损
  - name: session_scope
    description: 可选，regular 或 extended
    default: regular
---
请为 {{.Arg "symbol"}} 配置“{{.Arg "goal"}}”相关提醒，并按下面顺序行动：
1. 先调用 get_quote 查看当前价格与时段。
2. 再调用 analyze_trend 或 analyze_positions_trends，确认买入区、止盈位、止损位是否合理。
3. 根据目标创建 create_signal_alert：
   - 买入区：trend + in_buy_zone
   - 接近止盈：trend + near_take_profit
   - 跌破止损：trend + below_stop_loss
   - 纯价格阈值：price + above/below
4. 创建提醒时明确写入 session_scope={{.Arg "session_scope"}}，避免把常规盘规则和扩展时段规则混在一起。

如果发现已有提醒与当前目标冲突，请先列出 list_signal_alerts，再决定是删除重建还是保留。
