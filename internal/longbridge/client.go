package longbridge

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/config"
	"github.com/longportapp/openapi-go/quote"
	"github.com/longportapp/openapi-go/trade"
)

type Client struct {
	quoteCtx *quote.QuoteContext
	tradeCtx *trade.TradeContext
}

func NewClient(appKey, appSecret, accessToken string) (*Client, error) {
	conf, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	conf.AppKey = appKey
	conf.AppSecret = appSecret
	conf.AccessToken = accessToken

	// Create quote context
	quoteCtx, err := quote.NewFromCfg(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create quote context: %w", err)
	}

	// Create trade context
	tradeCtx, err := trade.NewFromCfg(conf)
	if err != nil {
		quoteCtx.Close()
		return nil, fmt.Errorf("failed to create trade context: %w", err)
	}

	return &Client{
		quoteCtx: quoteCtx,
		tradeCtx: tradeCtx,
	}, nil
}

func (c *Client) Close() {
	if c.quoteCtx != nil {
		c.quoteCtx.Close()
	}
	if c.tradeCtx != nil {
		c.tradeCtx.Close()
	}
}

// Quote API

// GetQuote returns real-time quotes
func (c *Client) GetQuote(ctx context.Context, symbols []string) ([]*quote.Quote, error) {
	return c.quoteCtx.RealtimeQuote(ctx, symbols)
}

// GetQuoteInfo returns static info for securities
func (c *Client) GetQuoteInfo(ctx context.Context, symbols []string) ([]*quote.StaticInfo, error) {
	return c.quoteCtx.StaticInfo(ctx, symbols)
}

// GetDepth returns depth data
func (c *Client) GetDepth(ctx context.Context, symbol string) (*quote.SecurityDepth, error) {
	return c.quoteCtx.RealtimeDepth(ctx, symbol)
}

// GetTrades returns trade data
func (c *Client) GetTrades(ctx context.Context, symbol string, count int32) ([]*quote.Trade, error) {
	return c.quoteCtx.RealtimeTrades(ctx, symbol)
}

// GetCandlesticks returns candlestick data
func (c *Client) GetCandlesticks(ctx context.Context, symbol string, period quote.Period, count int32) ([]*quote.Candlestick, error) {
	return c.quoteCtx.Candlesticks(ctx, symbol, period, count, quote.AdjustType(0))
}

// Trade API

// SubmitOrder submits a trade order
func (c *Client) SubmitOrder(ctx context.Context, order *trade.SubmitOrder) (string, error) {
	return c.tradeCtx.SubmitOrder(ctx, order)
}

// CancelOrder cancels an order
func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	return c.tradeCtx.CancelOrder(ctx, orderID)
}

// GetOrders returns today's orders
func (c *Client) GetOrders(ctx context.Context) ([]*trade.Order, error) {
	params := &trade.GetTodayOrders{}
	return c.tradeCtx.TodayOrders(ctx, params)
}

// GetOrderDetail returns order detail
func (c *Client) GetOrderDetail(ctx context.Context, orderID string) (trade.OrderDetail, error) {
	return c.tradeCtx.OrderDetail(ctx, orderID)
}

// GetPositions returns stock positions
func (c *Client) GetPositions(ctx context.Context) ([]*trade.StockPositionChannel, error) {
	return c.tradeCtx.StockPositions(ctx, nil)
}

// GetAccountInfo returns account balance
func (c *Client) GetAccountInfo(ctx context.Context) ([]*trade.AccountBalance, error) {
	params := &trade.GetAccountBalance{}
	return c.tradeCtx.AccountBalance(ctx, params)
}

// GetHistoryExecutions returns history executions
func (c *Client) GetHistoryExecutions(ctx context.Context, symbol string) ([]*trade.Execution, error) {
	params := &trade.GetHistoryExecutions{
		Symbol: symbol,
	}
	return c.tradeCtx.HistoryExecutions(ctx, params)
}
