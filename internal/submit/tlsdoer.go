package submit

import (
	"context"
	"io"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// ChromeDoer sends requests with a Chrome TLS fingerprint, which Cloudflare requires.
type ChromeDoer struct{ client tls_client.HttpClient }

func NewChromeDoer() (*ChromeDoer, error) {
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithNotFollowRedirects(),
	)
	return &ChromeDoer{c}, err
}

func (d *ChromeDoer) Do(ctx context.Context, r Req) (Resp, error) {
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, body)
	if err != nil {
		return Resp{}, err
	}
	req.Header = http.Header{}
	var order []string
	for _, k := range []string{"accept", "accept-language", "content-type", "origin", "referer", "cookie", "user-agent"} {
		if v, ok := r.Header[k]; ok {
			req.Header[k] = []string{v}
			order = append(order, k)
		}
	}
	req.Header[http.HeaderOrderKey] = order
	resp, err := d.client.Do(req)
	if err != nil {
		return Resp{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return Resp{Status: resp.StatusCode, Location: resp.Header.Get("location"), Body: b}, err
}
