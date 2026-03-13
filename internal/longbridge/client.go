package longbridge

import (
	"context"
	"fmt"
	"time"

	"github.com/longportapp/openapi-go/config"
	lhttp "github.com/longportapp/openapi-go/http"
	"github.com/longportapp/openapi-go/quote"
	"github.com/longportapp/openapi-go/trade"
)

type Client struct {
	httpClient *lhttp.Client
	quoteCtx   *quote.QuoteContext
	tradeCtx   *trade.TradeContext
}

func NewClient(appKey, appSecret, accessToken string) (*Client, error) {
	conf, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	conf.AppKey = appKey
	conf.AppSecret = appSecret
	conf.AccessToken = accessToken
	conf.SetLogger(newSDKLogger(conf.LogLevel))

	httpClient, err := lhttp.NewFromCfg(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

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
		httpClient: httpClient,
		quoteCtx:   quoteCtx,
		tradeCtx:   tradeCtx,
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

// GetQuote returns pull-based real-time quotes.
// RealtimeQuote only reads the local subscription cache and returns empty values
// when the symbol has not been subscribed first.
func (c *Client) GetQuote(ctx context.Context, symbols []string) ([]*quote.SecurityQuote, error) {
	return c.quoteCtx.Quote(ctx, symbols)
}

// GetQuoteInfo returns static info for securities
func (c *Client) GetQuoteInfo(ctx context.Context, symbols []string) ([]*quote.StaticInfo, error) {
	return c.quoteCtx.StaticInfo(ctx, symbols)
}

func (c *Client) GetStockNews(ctx context.Context, symbol string) ([]*StockNewsItem, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	var resp stockNewsResponse
	if err := c.httpClient.Get(ctx, "/v1/content/"+symbol+"/news", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetDepth returns depth data
func (c *Client) GetDepth(ctx context.Context, symbol string) (*quote.SecurityDepth, error) {
	return c.quoteCtx.Depth(ctx, symbol)
}

// GetTrades returns trade data
func (c *Client) GetTrades(ctx context.Context, symbol string, count int32) ([]*quote.Trade, error) {
	return c.quoteCtx.Trades(ctx, symbol, count)
}

// GetCandlesticks returns candlestick data
func (c *Client) GetCandlesticks(ctx context.Context, symbol string, period quote.Period, count int32) ([]*quote.Candlestick, error) {
	return c.quoteCtx.Candlesticks(ctx, symbol, period, count, quote.AdjustTypeNo)
}

func (c *Client) GetHistoryCandlesticksByDate(ctx context.Context, symbol string, period quote.Period, startDate, endDate *time.Time) ([]*quote.Candlestick, error) {
	return c.quoteCtx.HistoryCandlesticksByDate(ctx, symbol, period, quote.AdjustTypeNo, startDate, endDate)
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
func (c *Client) GetOrders(ctx context.Context, status []trade.OrderStatus) ([]*trade.Order, error) {
	params := &trade.GetTodayOrders{
		Status: status,
	}
	return c.tradeCtx.TodayOrders(ctx, params)
}

// GetTodayExecutions returns today's executions for the current trading day.
func (c *Client) GetTodayExecutions(ctx context.Context, symbol, orderID string) ([]*trade.Execution, error) {
	params := &trade.GetTodayExecutions{
		Symbol:  symbol,
		OrderId: orderID,
	}
	return c.tradeCtx.TodayExecutions(ctx, params)
}

func (c *Client) GetHistoryOrders(ctx context.Context, symbol string, status []trade.OrderStatus, startAt, endAt time.Time) ([]*trade.Order, bool, error) {
	params := &trade.GetHistoryOrders{
		Symbol: symbol,
		Status: status,
	}
	if !startAt.IsZero() {
		params.StartAt = startAt.Unix()
	}
	if !endAt.IsZero() {
		params.EndAt = endAt.Unix()
	}
	return c.tradeCtx.HistoryOrders(ctx, params)
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
func (c *Client) GetHistoryExecutions(ctx context.Context, symbol string, startAt, endAt time.Time) ([]*trade.Execution, error) {
	params := &trade.GetHistoryExecutions{
		Symbol:  symbol,
		StartAt: startAt,
		EndAt:   endAt,
	}
	return c.tradeCtx.HistoryExecutions(ctx, params)
}
