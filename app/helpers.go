package app

import (
	"io"
	"log"
	"net/http"

	"golang.org/x/net/context"
)

// closeC closes io.Closer and logs error if any.
// It's useful in place where we don't want to know result of close,
// but need to prevent annoying gometalinter alerts
func closeC(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Printf("ERR: failed to close: %+v", err)
	}
}

func (d *Downloader) buildRequest(ctx context.Context, link string) *http.Request {
	// not checking error here as we are definitely sure
	// that we use only correct urls and request is only GET
	req, _ := http.NewRequest("GET", link, nil)

	ctx, _ = context.WithTimeout(ctx, d.timeout)
	req = req.WithContext(ctx)

	return req
}
