package control

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"bkplight.local/bkplight/internal/safeerr"
)

type Client struct {
	socket string
	http   *http.Client
}

func NewClient(socket string) *Client {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: requestTimeout}).DialContext(ctx, "unix", socket)
	}, MaxIdleConns: 1, MaxIdleConnsPerHost: 1, MaxConnsPerHost: 32, IdleConnTimeout: 30 * requestTimeout, ResponseHeaderTimeout: requestTimeout, MaxResponseHeaderBytes: 64 << 10, DisableKeepAlives: true}
	return &Client{socket: socket, http: &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return internal() }}}
}

// Do accepts only an origin-relative API path. The transport cannot dial TCP,
// use an environment proxy, or follow a redirect to another target.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if !validSocket(c.socket) || len(body) > maxBody || (method != http.MethodGet && method != http.MethodPost) {
		return nil, invalid()
	}
	u, err := url.ParseRequestURI(path)
	if err != nil {
		return nil, invalid()
	}
	if !routePath(&http.Request{URL: u}) || strings.HasPrefix(path, "//") {
		return nil, invalid()
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, invalid()
	}
	if method == http.MethodGet && u.Path == "/v1/runs" {
		if _, _, err := historyQuery(q); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost"+path, bytes.NewReader(body))
	if err != nil {
		return nil, invalid()
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, unavailable()
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		return nil, unavailable()
	}
	if len(raw) > maxResponse {
		return nil, internal()
	}
	var e envelope
	if strictJSON(raw, &e) != nil || (e.RequestID != "" && !requestIDPattern.MatchString(e.RequestID)) || (e.OperationID != "" && !runIDPattern.MatchString(e.OperationID)) {
		return nil, internal()
	}
	if e.Status == "error" {
		if e.Error == nil || len(e.Data) != 0 || e.OperationID != "" {
			return nil, internal()
		}
		public := safeerr.Public(&safeerr.Error{Code: e.Error.Code, Retryable: e.Error.Retryable})
		status, _ := errorStatus(public.Code)
		if status != response.StatusCode {
			return nil, internal()
		}
		return nil, &public
	}
	if e.Status != "ok" || e.Error != nil || len(e.Data) == 0 {
		return nil, internal()
	}
	// Decode the route's closed safe view and re-encode it. Neither arbitrary
	// response fields nor raw diagnostic text reach CLI output.
	data, err := clientData(method, u.Path, e.Data)
	if err != nil {
		return nil, internal()
	}
	e.Data = data
	if u.Path == "/v1/health" && method == http.MethodGet {
		var h Health
		_ = json.Unmarshal(data, &h)
		if !h.Ready && response.StatusCode == 503 {
			encoded, _ := json.Marshal(e)
			return encoded, unavailable()
		}
		if !h.Ready || response.StatusCode != 200 {
			return nil, internal()
		}
	} else if response.StatusCode != 200 {
		return nil, internal()
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return nil, internal()
	}
	return encoded, nil
}

func clientData(method, path string, raw []byte) (json.RawMessage, error) {
	var value any
	switch {
	case method == http.MethodGet && path == "/v1/health":
		var h Health
		if strictJSON(raw, &h) != nil || h.Scope != "foundation" || h.Scheduler != "not_implemented" || h.BackupEngine != "not_implemented" {
			return nil, internal()
		}
		value = safeHealth(h)
	case method == http.MethodGet && path == "/v1/jobs":
		jobs := []JobView{}
		if strictJSON(raw, &jobs) != nil || jobs == nil {
			return nil, internal()
		}
		for _, j := range jobs {
			if !labelPattern.MatchString(j.JobID) || !labelPattern.MatchString(j.SourceID) || j.Engine != "not_implemented" {
				return nil, internal()
			}
		}
		value = jobs
	case method == http.MethodGet && path == "/v1/runs":
		var page RunPage
		if strictJSON(raw, &page) != nil || len(page.Items) > 200 || !validCursor(page.Next) {
			return nil, internal()
		}
		for _, r := range page.Items {
			if !validRunView(r) {
				return nil, internal()
			}
		}
		value = page
	case strings.HasPrefix(path, "/v1/runs/"):
		var r RunView
		if strictJSON(raw, &r) != nil || !validRunView(r) {
			return nil, internal()
		}
		value = r
	default:
		return nil, internal()
	}
	return json.Marshal(value)
}
func validRunView(r RunView) bool {
	if !runIDPattern.MatchString(r.ID) || r.Version < 0 {
		return false
	}
	switch r.State {
	case "queued", "preflight", "dumping", "delivering", "succeeded", "degraded", "failed", "cancelled", "interrupted":
	default:
		return false
	}
	if r.ErrorCode != "" && safeerr.Public(&safeerr.Error{Code: safeerr.Code(r.ErrorCode)}).Code != safeerr.Code(r.ErrorCode) {
		return false
	}
	return true
}
