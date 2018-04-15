package main

import (
	"github.com/prometheus/common/log"
	"github.com/scukonick/tegw/app"
)

func main() {
	d := app.NewDownloader()
	err := d.Run("http://127.0.0.1:8000")
	if err != nil {
		log.Fatalf("run failed: %+v", err)
	}
}
