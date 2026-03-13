package okx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client wraps basic OKX REST API access with signing.
// Only a subset of public/account/trade endpoints are implemented to match
// existing LongBridge parity (行情、下单、撤单、账户余额等).
type Client struct {
	httpClient *http.Client
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string

	mu         sync.Mutex
	timeOffset time.Duration
	lastSync   time.Time
}

// NewClient creates a new OKX client. baseURL is optional; defaults to https://www.okx.com.
func NewClient(apiKey, secretKey, passphrase, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://www.okx.com"
	}
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
		secretKey:  strings.TrimSpace(secretKey),
		passphrase: strings.TrimSpace(passphrase),
		baseURL:    strings.TrimRight(baseURL, "/"),
	}
}

// Response is OKX common envelope.
type Response[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []T    `json:"data"`
}

// Ticker represents /market/ticker response item.
type Ticker struct {
	InstType  string `json:"instType"`
	InstID    string `json:"instId"`
	Last      string `json:"last"`
	LastSz    string `json:"lastSz"`
	AskPx     string `json:"askPx"`
	AskSz     string `json:"askSz"`
	BidPx     string `json:"bidPx"`
	BidSz     string `json:"bidSz"`
	Open24h   string `json:"open24h"`
	High24h   string `json:"high24h"`
	Low24h    string `json:"low24h"`
	Vol24h    string `json:"vol24h"`
	VolCcy24h string `json:"volCcy24h"`
	Ts        string `json:"ts"`
}

// OrderBook represents /market/books item.
type OrderBook struct {
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
	Ts   string     `json:"ts"`
}

// Trade represents /market/trades item.
type Trade struct {
	InstID string `json:"instId"`
	Side   string `json:"side"`
	Sz     string `json:"sz"`
	Px     string `json:"px"`
	Ts     string `json:"ts"`
}

// Candle represents a single candlestick.
type Candle struct {
	Ts          int64
	O           string
	H           string
	L           string
	C           string
	Vol         string
	VolCcy      string
	VolCcyQuote string
}

// Balance represents account balance item.
type Balance struct {
	Ccy       string `json:"ccy"`
	CashBal   string `json:"cashBal"`
	Upl       string `json:"upl"`
	UplLiab   string `json:"uplLiab"`
	FrozenBal string `json:"frozenBal"`
	Eq        string `json:"eq"`
	AvailEq   string `json:"availEq"`
}

// OrderResponse represents trade order response.
type OrderResponse struct {
	OrdID   string `json:"ordId"`
	ClOrdID string `json:"clOrdId"`
	InstID  string `json:"instId"`
	SCode   string `json:"sCode"`
	SMsg    string `json:"sMsg"`
}

func (c *Client) GetTicker(ctx context.Context, instID string) (*Ticker, error) {
	var resp Response[Ticker]
	err := c.get(ctx, "/api/v5/market/ticker", url.Values{"instId": {instID}}, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return &resp.Data[0], nil
}

func (c *Client) GetOrderBook(ctx context.Context, instID string, depth int) (*OrderBook, error) {
	if depth <= 0 {
		depth = 5
	}
	vals := url.Values{"instId": {instID}, "sz": {fmt.Sprintf("%d", depth)}}
	var resp Response[OrderBook]
	err := c.get(ctx, "/api/v5/market/books", vals, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return &resp.Data[0], nil
}

func (c *Client) GetTrades(ctx context.Context, instID string, limit int) ([]Trade, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	vals := url.Values{"instId": {instID}, "limit": {fmt.Sprintf("%d", limit)}}
	var resp Response[Trade]
	err := c.get(ctx, "/api/v5/market/trades", vals, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) GetCandles(ctx context.Context, instID, bar string, limit int) ([]Candle, error) {
	if bar == "" {
		bar = "1m"
	}
	if limit <= 0 || limit > 300 {
		limit = 100
	}
	vals := url.Values{"instId": {instID}, "bar": {bar}, "limit": {fmt.Sprintf("%d", limit)}}
	// /market/candles returns [][]string
	var raw struct {
		Code string     `json:"code"`
		Msg  string     `json:"msg"`
		Data [][]string `json:"data"`
	}
	if err := c.get(ctx, "/api/v5/market/candles", vals, &raw); err != nil {
		return nil, err
	}
	if raw.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", raw.Code, raw.Msg)
	}
	candles := make([]Candle, 0, len(raw.Data))
	for _, arr := range raw.Data {
		if len(arr) < 6 {
			continue
		}
		ts, _ := parseInt64(arr[0])
		candles = append(candles, Candle{
			Ts:          ts,
			O:           arr[1],
			H:           arr[2],
			L:           arr[3],
			C:           arr[4],
			Vol:         arr[5],
			VolCcy:      firstOrEmpty(arr, 6),
			VolCcyQuote: firstOrEmpty(arr, 7),
		})
	}
	return candles, nil
}

// Account APIs

type balanceResponse struct {
	AdjEq   string    `json:"adjEq"`
	Details []Balance `json:"details"`
	UTime   string    `json:"uTime"`
}

func (c *Client) GetBalances(ctx context.Context) ([]Balance, error) {
	var resp Response[balanceResponse]
	if err := c.getAuth(ctx, http.MethodGet, "/api/v5/account/balance", nil, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0].Details, nil
}

// Trade / Order APIs

type PlaceOrderRequest struct {
	InstID  string `json:"instId"`
	TdMode  string `json:"tdMode"`
	ClOrdID string `json:"clOrdId,omitempty"`
	Side    string `json:"side"`
	OrdType string `json:"ordType"`
	Sz      string `json:"sz"`
	Px      string `json:"px,omitempty"`
}

func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*OrderResponse, error) {
	var resp Response[OrderResponse]
	if err := c.postAuth(ctx, "/api/v5/trade/order", req, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return &resp.Data[0], nil
}

type CancelOrderRequest struct {
	InstID  string `json:"instId"`
	OrdID   string `json:"ordId,omitempty"`
	ClOrdID string `json:"clOrdId,omitempty"`
}

func (c *Client) CancelOrder(ctx context.Context, req CancelOrderRequest) (*OrderResponse, error) {
	var resp Response[OrderResponse]
	if err := c.postAuth(ctx, "/api/v5/trade/cancel-order", req, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return &resp.Data[0], nil
}

type OrderItem struct {
	InstID    string `json:"instId"`
	OrdID     string `json:"ordId"`
	ClOrdID   string `json:"clOrdId"`
	State     string `json:"state"`
	Side      string `json:"side"`
	OrdType   string `json:"ordType"`
	AvgPx     string `json:"avgPx"`
	Px        string `json:"px"`
	Sz        string `json:"sz"`
	AccFillSz string `json:"accFillSz"`
	FillSz    string `json:"fillSz"`
	Ts        string `json:"uTime"`
}

func (c *Client) GetOpenOrders(ctx context.Context, instType string) ([]OrderItem, error) {
	if instType == "" {
		instType = "SPOT"
	}
	vals := url.Values{"instType": {instType}}
	var resp Response[OrderItem]
	if err := c.getAuth(ctx, http.MethodGet, "/api/v5/trade/orders-pending", vals, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) GetOrderHistory(ctx context.Context, instType string, limit int) ([]OrderItem, error) {
	if instType == "" {
		instType = "SPOT"
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	vals := url.Values{"instType": {instType}, "limit": {fmt.Sprintf("%d", limit)}}
	var resp Response[OrderItem]
	if err := c.getAuth(ctx, http.MethodGet, "/api/v5/trade/orders-history", vals, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("okx error %s: %s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

// ----- low-level helpers -----

func (c *Client) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, query, nil, false, out)
}

func (c *Client) getAuth(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	return c.do(ctx, method, path, query, body, true, out)
}

func (c *Client) postAuth(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, nil, body, true, out)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body interface{}, sign bool, out interface{}) error {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	requestPath := path
	if len(query) > 0 {
		encoded := query.Encode()
		requestPath = requestPath + "?" + encoded
	}

	reqURL := c.baseURL + requestPath
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sign {
		if err := c.signRequest(ctx, req, requestPath, string(bodyBytes)); err != nil {
			return err
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("okx http %d: %s", resp.StatusCode, string(respBytes))
	}

	if out != nil {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("failed to decode okx response: %w", err)
		}
	}
	return nil
}

func (c *Client) signRequest(ctx context.Context, req *http.Request, requestPath, body string) error {
	if c.apiKey == "" || c.secretKey == "" || c.passphrase == "" {
		return fmt.Errorf("missing okx credentials")
	}
	ts, err := c.timestampForSign(ctx)
	if err != nil {
		return fmt.Errorf("get server time: %w", err)
	}
	preSign := ts + strings.ToUpper(req.Method) + requestPath + body

	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(preSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("OK-ACCESS-KEY", c.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", sign)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)
	req.Header.Set("x-simulated-trading", "0")
	return nil
}

// timestampForSign returns an RFC3339Nano timestamp aligned with OKX server time.
// OKX允许最大 30 秒偏差，仍建议与服务器同步。
func (c *Client) timestampForSign(ctx context.Context) (string, error) {
	now := time.Now()
	c.mu.Lock()
	needSync := c.lastSync.IsZero() || now.Sub(c.lastSync) > 55*time.Second
	offset := c.timeOffset
	c.mu.Unlock()

	if needSync {
		if err := c.syncServerTime(ctx); err != nil {
			// 如果同步失败，回退本地时间，避免阻塞请求
			return now.UTC().Format(time.RFC3339Nano), nil
		}
		c.mu.Lock()
		offset = c.timeOffset
		c.mu.Unlock()
	}
	return now.Add(offset).UTC().Format(time.RFC3339Nano), nil
}

func (c *Client) syncServerTime(ctx context.Context) error {
	type timeResp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			TS string `json:"ts"`
		} `json:"data"`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v5/public/time", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("server time http %d: %s", resp.StatusCode, string(body))
	}
	var tr timeResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return err
	}
	if tr.Code != "0" || len(tr.Data) == 0 {
		return fmt.Errorf("server time error %s: %s", tr.Code, tr.Msg)
	}
	tsMillis, err := strconv.ParseInt(tr.Data[0].TS, 10, 64)
	if err != nil {
		return fmt.Errorf("parse server ts: %w", err)
	}
	serverNow := time.UnixMilli(tsMillis)
	now := time.Now()

	c.mu.Lock()
	c.timeOffset = serverNow.Sub(now)
	c.lastSync = now
	c.mu.Unlock()
	return nil
}

func parseInt64(s string) (int64, error) {
	if strings.TrimSpace(s) == "" {
		return 0, fmt.Errorf("empty")
	}
	// ts is in milliseconds string
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

func firstOrEmpty(arr []string, idx int) string {
	if len(arr) > idx {
		return arr[idx]
	}
	return ""
}
