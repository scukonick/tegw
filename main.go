package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/scukonick/tegw/app"
)

func main() {
	d := app.NewDownloader()
	go func() {
		err := d.Run("http://127.0.0.1:8000")
		if err != nil {
			log.Fatalf("run failed: %+v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	log.Println("Received stop signal, exiting...")
	d.Stop()
	log.Println("shut down")
}
