package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/scukonick/tegw/app"
)

var baseURL string
var outDir string
var stateDir string

func init() {
	flag.StringVar(&baseURL, "baseURL", "http://google.com", "url to start downloads")
	flag.StringVar(&outDir, "outDir", ".", "where to store downloaded docs")
	flag.StringVar(&stateDir, "stateDir", ".", "where to store state")
	flag.Parse()
}

func main() {
	d := app.NewDownloader(outDir, stateDir)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		<-c
		log.Println("Received stop signal, exiting...")
		d.Stop()
	}()

	err := d.Run(baseURL)
	if err != nil {
		log.Fatalf("run failed: %+v", err)
	}

}
