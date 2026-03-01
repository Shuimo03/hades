package longbridge

import (
	"context"
	"fmt"

	"github.com/longportapp/openapi-go/config"
	"github.com/longportapp/openapi-go/quote"
	"github.com/longportapp/openapi-go/trade"
)

type Client struct {
	quoteCtx *quote.Context
	tradeCtx *trade.Context
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

func (c *Client) GetQuote(ctx context.Context, symbols []string) ([]*quote.Quote, error) {
	return c.quoteCtx.Quote(ctx, symbols)
}

func (c *Client) GetQuoteInfo(ctx context.Context, symbols []string) ([]*quote.StaticInfo, error) {
	return c.quoteCtx.StaticInfo(ctx, symbols)
}

func (c *Client) GetDepth(ctx context.Context, symbol string, size int) (*quote.Depth, error) {
	return c.quoteCtx.Depth(ctx, symbol, size)
}

func (c *Client) GetTrades(ctx context.Context, symbol string, start int64, end int64, count int) ([]*quote.Trade, error) {
	return c.quoteCtx.Trades(ctx, symbol, start, end, count)
}

func (c *Client) GetCandlesticks(ctx context.Context, symbol string, period quote.CandlestickPeriod, start int64, end int64, count int) ([]*quote.Candlestick, error) {
	return c.quoteCtx.Candlesticks(ctx, symbol, period, start, end, count)
}

// Trade API

func (c *Client) SubmitOrder(ctx context.Context, order *trade.SubmitOrderRequest) (string, error) {
	return c.tradeCtx.SubmitOrder(ctx, order)
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	return c.tradeCtx.CancelOrder(ctx, orderID)
}

func (c *Client) GetOrders(ctx context.Context, status string) ([]*trade.Order, error) {
	return c.tradeCtx.Orders(ctx, status)
}

func (c *Client) GetOrderDetail(ctx context.Context, orderID string) (*trade.OrderDetail, error) {
	return c.tradeCtx.OrderDetail(ctx, orderID)
}

func (c *Client) GetPositions(ctx context.Context) ([]*trade.Position, error) {
	return c.tradeCtx.Positions(ctx)
}

func (c *Client) GetAccountInfo(ctx context.Context) (*trade.AccountInfo, error) {
	return c.tradeCtx.AccountInfo(ctx)
}

func (c *Client) GetHistoryExecutions(ctx context.Context, symbol string, start int64, end int64) ([]*trade.Execution, error) {
	return c.tradeCtx.HistoryExecution(ctx, symbol, start, end)
}
