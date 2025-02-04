package databox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	// DateFormat will be converted to '2006-01-02 00:00:00+00:00'.
	DateFormat = "2006-01-02"
	// DateTimeFormat will be converted to '2006-01-02 15:04:05+00:00'.
	DateTimeFormat = "2006-01-02 15:04:05"
	// DateTimeTZFormat will be stored as was sent.
	DateTimeTZFormat = "2006-01-02 15:04:05-07:00"
)

const (
	apiURL        = "https://push.databox.com"
	clientVersion = "2.1.0"
)

// Client struct holds push token and host to Databox service
type Client struct {
	PushToken  string
	PushHost   string
	HTTPClient *http.Client
}

// KPI struct holds information about item in push request
type KPI struct {
	// Key is key of the metric.
	Key string
	// Value is value of the metric.
	Value float32
	// Metrics can contain other metrics if you want to send multiple of them with same
	// attributes.
	Metrics map[string]float32
	// Date specifies data or datetime of the metric. It's optional. The Date should
	// have been formatted as Date in DateFormat, or datetime in DateTimeTZFormat.
	// Timezone is UTC if DateFormat or DateTimeFormat is used.
	Date string
	// Unit describes value of the metric. It's optional. Any string can be used as
	// unit.
	Unit string
	// Attributes can contain arbitrary information for metrics. If you send an
	// attribute along with multiple metrics, the attribute will be added to each
	// metric.
	Attributes map[string]interface{}
}

// KPIWrap struct is just a wrapper around KPI with root key "data"
type KPIWrap struct {
	Data []map[string]interface{} `json:"data"`
	Meta map[string]interface{}   `json:"meta,omitempty"`
}

// ResponseStatus struct is for different response variations
type ResponseStatus struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// PushRequest struct holds information about Request returned from LastPush request
type PushRequest struct {
	Date   string   `json:"date"`
	Body   KPIWrap  `json:"body"`
	Errors []string `json:"errors"`
}

// PushResponse struct holds information about Response returned from LastPush request
type PushResponse struct {
	Date string         `json:"date"`
	Body ResponseStatus `json:"body"`
}

// LastPush struct holds item information from LastPush request
type LastPush struct {
	Request  PushRequest  `json:"request"`
	Response PushResponse `json:"response"`
	Metrics  []string     `json:"metrics"`
}

// NewClient returns object for making calls against a Databox service.
func NewClient(pushToken string) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// We use only one host: push.databox.com
	transport.MaxIdleConnsPerHost = transport.MaxIdleConns

	return &Client{
		PushToken: pushToken,
		PushHost:  apiURL,
		HTTPClient: &http.Client{
			Transport: transport,
		},
	}
}

func (c *Client) postRequest(ctx context.Context, path string, payload []byte) ([]byte, error) {
	userAgent := "databox-go/" + clientVersion
	accept := "application/vnd.databox.v" + strings.Split(clientVersion, ".")[0] + "+json"
	request, err := http.NewRequestWithContext(ctx, "POST", apiURL+path, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request object: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", accept)
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(c.PushToken, "")

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("executing HTTP request: %w", err)
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return data, fmt.Errorf("reading response body: %w", err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		var responseStatus = &ResponseStatus{}
		if err := json.Unmarshal(data, &responseStatus); err != nil {
			return nil, fmt.Errorf("can't unmarshal data[%s]: %w", string(data), err)
		}
		return nil, errors.New(responseStatus.Type + ": " + responseStatus.Message)
	}

	return data, nil
}

func (c *Client) getRequest(ctx context.Context, path string) ([]byte, error) {
	userAgent := "databox-go/" + clientVersion
	accept := "application/vnd.databox.v" + strings.Split(clientVersion, ".")[0] + "+json"
	request, err := http.NewRequestWithContext(ctx, "GET", apiURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request object: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", accept)
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(c.PushToken, "")

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("executing HTTP request: %w", err)
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return data, nil
}

// LastPushes returns n last pushes from Databox service.
func (c *Client) LastPushes(n int) ([]LastPush, error) {
	return c.LastPushesCtx(context.Background(), n)
}

// LastPushesCtx returns n last pushes from Databox service. It terminates the
// request on context cancellation.
func (c *Client) LastPushesCtx(ctx context.Context, n int) ([]LastPush, error) {
	path := fmt.Sprintf("/lastpushes?limit=%d", n)
	response, err := c.getRequest(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("requesting /lastpushes from API: %w", err)
	}

	lastPushes := make([]LastPush, 0)
	if err := json.Unmarshal(response, &lastPushes); err != nil {
		return nil, fmt.Errorf("can't unmarshal response[%s]: %w", string(response), err)
	}

	return lastPushes, nil
}

// LastPush returns latest push from Databox service.
func (c *Client) LastPush() (LastPush, error) {
	return c.LastPushCtx(context.Background())
}

// LastPushCtx returns latest push from Databox service. It terminates the
// request on context cancellation.
func (c *Client) LastPushCtx(ctx context.Context) (LastPush, error) {
	lastPushes, err := c.LastPushesCtx(ctx, 1)
	if err != nil {
		return LastPush{}, err
	}
	if len(lastPushes) == 0 {
		return LastPush{}, errors.New("no last push")
	}
	return lastPushes[0], nil
}

// Push makes push request against Databox service.
func (c *Client) Push(kpi *KPI) (*ResponseStatus, error) {
	return c.PushCtx(context.Background(), kpi)
}

// PushCtx makes push request against Databox service. It terminates the
// request on context cancellation.
func (c *Client) PushCtx(ctx context.Context, kpi *KPI) (*ResponseStatus, error) {
	payload, err := serializeKPIs([]KPI{*kpi}, false)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	response, err := c.postRequest(ctx, "/", payload)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var responseStatus = &ResponseStatus{}
	if err := json.Unmarshal(response, &responseStatus); err != nil {
		return nil, fmt.Errorf("can't unmarshal respoonse[%s]: %w", string(response), err)
	}

	return responseStatus, nil
}

// TODO document forcePush attribute
//  It seems like secret option implemented in Go and Python SDK only.
//  Not documented in both.
//  https://github.com/databox/databox-python/blob/master/databox/__init__.py#L108

// InsertAll makes insertAll request against Databox service. It terminates the
// request on context cancellation.
func (c *Client) InsertAll(ctx context.Context, kpis []KPI, forcePush bool) (*ResponseStatus, error) {
	payload, err := serializeKPIs(kpis, forcePush)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	response, err := c.postRequest(ctx, "/", payload)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	var responseStatus = &ResponseStatus{}
	if err := json.Unmarshal(response, &responseStatus); err != nil {
		return nil, fmt.Errorf("can't unmarshal respoonse[%s]: %w", string(response), err)
	}

	return responseStatus, nil
}

// ToJSONData serializes KPI to json
func (kpi *KPI) ToJSONData() map[string]interface{} {
	var payload = make(map[string]interface{})

	// store attributes first so it can't overwrite other values.
	for key, value := range kpi.Attributes {
		payload[key] = value
	}

	for key, value := range kpi.Metrics {
		payload["$"+key] = value
	}

	if kpi.Key != "" {
		payload["$"+kpi.Key] = kpi.Value
	}

	if kpi.Date != "" {
		payload["date"] = kpi.Date
	}

	if kpi.Unit != "" {
		payload["unit"] = kpi.Unit
	}

	return payload
}

// serializeKPIs traverse all kpis and return json representation
func serializeKPIs(kpis []KPI, forcePush bool) ([]byte, error) {
	wrap := KPIWrap{
		Data: make([]map[string]interface{}, 0),
	}
	if forcePush {
		wrap.Meta = map[string]interface{}{
			"ensure_unique": true,
		}
	}

	for _, kpi := range kpis {
		wrap.Data = append(wrap.Data, kpi.ToJSONData())
	}

	return json.Marshal(wrap)
}
