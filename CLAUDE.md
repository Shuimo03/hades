# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go (Golang) MCP (Model Context Protocol) server that integrates with LongBridge OpenAPI for Hong Kong/US stock trading.

## Project Structure

```
hades/
├── cmd/server/main.go       # Main entry point
├── internal/
│   ├── config/             # Configuration loading
│   ├── longbridge/        # LongBridge API client wrapper
│   ├── mcp/               # MCP HTTP server implementation
│   └── tools/             # MCP tools (quote, account, order, history)
├── config.yaml.example     # Configuration example
└── bin/server             # Compiled binary
```

## Development Commands

- Build: `go build -o bin/server ./cmd/server`
- Run: `./bin/server -config config.yaml`

## Available Tools

The MCP server provides the following tools:

### Quote Tools
- `get_quote` - Get real-time stock quotes
- `get_quote_info` - Get stock basic information
- `get_depth` - Get order book (买卖盘)
- `get_trades` - Get real-time trades

### Historical Data
- `get_candlesticks` - Get K-line/candlestick data

### Account Tools
- `get_account_info` - Get account balance information
- `get_positions` - Get current positions

### Order Tools
- `submit_order` - Submit a trade order
- `cancel_order` - Cancel an order
- `get_orders` - Get today's orders
- `get_order_detail` - Get order details
- `get_history_executions` - Get historical executions

## Configuration

Create a `config.yaml` file:

```yaml
app_key: "your_app_key"
app_secret: "your_app_secret"
access_token: "your_access_token"
server:
  host: "0.0.0.0"
  port: 8080
```

Or use environment variables:
- `LONGPORT_APP_KEY`
- `LONGPORT_APP_SECRET`
- `LONGPORT_ACCESS_TOKEN`

## Running the Server

```bash
./bin/server -config config.yaml
```

The server will start on `http://localhost:8080/mcp/`

## License

Mozilla Public License 2.0
