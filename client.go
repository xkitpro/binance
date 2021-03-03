package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/go-querystring/query"
)

type Security int

const (
	NONE Security = iota
	TRADE
	USER_DATA
	USER_STREAM
	MARKET_DATA
)

type EventType string

const (
	ORDER_TRADE_UPDATE EventType = "ORDER_TRADE_UPDATE"
)

const defaultBaseURL = "https://fapi.binance.com"

type NewOrderOptions struct {
	Symbol      string `json:"symbol" url:"symbol"`
	Side        string `json:"side" url:"side"`
	Type        string `json:"type" url:"type"`
	TimeInForce string `json:"timeInForce" url:"timeInForce"`
	Quantity    string `json:"quantity" url:"quantity"`
	Price       string `json:"price" url:"price"`
	ReduceOnly  string `json:"reduceOnly"`
}

type Client struct {
	baseURL *url.URL
	key     string
	secret  []byte
}

func (c *Client) NewOrder(opt *NewOrderOptions) error {
	req, err := c.NewRequest("POST", "/fapi/v1/order", opt, TRADE)
	if err != nil {
		return err
	}

	c.Do(req, nil)

	return nil
}

func NewClient(key, secret string) *Client {
	baseURL, _ := url.Parse(defaultBaseURL)

	return &Client{
		baseURL: baseURL,
		key:     key,
		secret:  []byte(secret),
	}
}

func (c *Client) NewRequest(method, path string, opt interface{}, sec Security) (*http.Request, error) {
	u, _ := c.baseURL.Parse(path)
	v, err := query.Values(opt)
	if err != nil {
		return nil, err
	}

	if sec == TRADE || sec == USER_DATA {
		u.RawQuery = c.sign(v)
	} else {
		u.RawQuery = v.Encode()
	}

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}

	if sec == USER_STREAM || sec == MARKET_DATA {
		req.Header.Add("X-MBX-APIKEY", c.key)
	}

	return req, err
}

func (c *Client) sign(v url.Values) string {
	v.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixNano()/int64(time.Millisecond)))

	qs := v.Encode()

	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(qs))
	s := fmt.Sprintf("signature=%x", mac.Sum(nil))

	return qs + "&" + s
}

func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()

	r := io.TeeReader(resp.Body, os.Stderr)

	if v != nil {
		err = json.NewDecoder(r).Decode(&v)
	}
	fmt.Fprintln(os.Stderr)

	return resp, err
}

type UserDataStream struct {
	ListenKey string `json:"listenKey"`
}

func (c *Client) StartUserDataStream() (*UserDataStream, *http.Response, error) {
	req, err := c.NewRequest("POST", "/fapi/v1/listenKey", nil, USER_STREAM)
	if err != nil {
		return nil, nil, err
	}

	r := new(UserDataStream)
	resp, err := c.Do(req, r)

	return r, resp, err
}

func (c *Client) KeepAliveUserDataStream() (*http.Response, error) {
	req, err := c.NewRequest("PUT", "/fapi/v1/listenKey", nil, USER_STREAM)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req, nil)

	return resp, err
}

func (c *Client) CloseUserDataStream() (*http.Response, error) {
	req, err := c.NewRequest("DELETE", "/fapi/v1/listenKey", nil, USER_STREAM)
	if err != nil {
		return nil, err
	}

	resp, err := c.Do(req, nil)

	return resp, err
}

type Candletick struct {
	OpenTime  int `json:"0"`
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int
}

type CandlestickDataOptions struct {
	Symbol    string `url:"symbol"`
	Interval  string `url:"interval"`
	StartTime int    `url:"startTime,omitempty"`
	EndTime   int    `url:"endTime,omitempty"`
	Limit     int    `url:"limit,omitempty"`
}

func (c *Client) CandlestickData(opt *CandlestickDataOptions) ([]Candletick, *http.Response, error) {
	req, err := c.NewRequest("GET", "/fapi/v1/klines", opt, MARKET_DATA)
	if err != nil {
		return nil, nil, err
	}

	var v []Candletick
	resp, err := c.Do(req, &v)

	return v, resp, err
}
