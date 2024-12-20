package httpclient

import (
	"context"
	"io"
	"net/http"
)

type Client interface {
	Do(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error)
}

// client implements the Client interface using net/http.
type httpclient struct {
	client *http.Client
}

// NewHttpClient creates a new instance of HttpClient.
func NewHttpClient(client *http.Client) Client {
	return &httpclient{client: client}
}

// Do sends an HTTP request with the specified method, URL, body, and headers.
func (c *httpclient) Do(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.client.Do(req)
}
