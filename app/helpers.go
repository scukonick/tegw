package app

import (
	"io"
	"log"
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
