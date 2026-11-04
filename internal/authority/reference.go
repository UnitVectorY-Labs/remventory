package authority

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// VisualReference is private recognition input, never a redistributed UI asset.
type VisualReference interface {
	Reference(context.Context, Ref) ([]byte, string, error)
}

func (p *Amiibo) Reference(ctx context.Context, ref Ref) ([]byte, string, error) {
	if !amiiboID.MatchString(ref.ID) {
		return nil, "", fmt.Errorf("invalid figure identifier")
	}
	var response amiiboResponse
	if err := p.HTTP.Get(ctx, "amiibo", p.base()+"?id="+ref.ID, &response); err != nil {
		return nil, "", err
	}
	for _, r := range response.Amiibo {
		if r.Head+r.Tail != ref.ID {
			continue
		}
		u, err := url.Parse(r.Image)
		suffix := "/images/icon_" + r.Head + "-" + r.Tail + ".png"
		if err != nil || u.Scheme != "https" || u.Host != "raw.githubusercontent.com" || (!strings.HasPrefix(u.Path, "/8bitDream/AmiiboAPI/") && !strings.HasPrefix(u.Path, "/N3evin/AmiiboAPI/")) || !strings.HasSuffix(u.Path, suffix) {
			return nil, "", fmt.Errorf("catalogue reference image is not from the approved source")
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, "", err
		}
		client := *p.HTTP.HTTP
		client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			if r.URL.Scheme != "https" || r.URL.Host != "raw.githubusercontent.com" || len(via) > 3 {
				return fmt.Errorf("reference redirect rejected")
			}
			return nil
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return nil, "", fmt.Errorf("reference image unavailable")
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
		if err != nil || len(data) > 2<<20 {
			return nil, "", fmt.Errorf("reference image exceeds recognition limit")
		}
		mime := http.DetectContentType(data)
		if mime != "image/png" && mime != "image/jpeg" {
			return nil, "", fmt.Errorf("invalid reference image")
		}
		return data, mime, nil
	}
	return nil, "", fmt.Errorf("source did not verify reference image")
}
