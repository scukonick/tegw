package app

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Downloader is a crawler which parses incoming url
// and stores text files to disk
type Downloader struct {
	urls      map[string]bool
	files     map[string]bool
	urlsCh    chan *url.URL
	filesCh   chan *url.URL
	stopCh    chan interface{}
	urlsLock  sync.RWMutex
	filesLock sync.RWMutex
	client    *http.Client
	limiter   chan interface{} // limits number of simultaneous downloads
	baseURL   *url.URL
	urlsWG    sync.WaitGroup
	wg        sync.WaitGroup
}

func NewDownloader() *Downloader {
	limiter := make(chan interface{}, 10)
	for i := 0; i < 10; i++ {
		limiter <- true
	}

	return &Downloader{
		urls:    make(map[string]bool, 100),
		files:   make(map[string]bool, 100),
		urlsCh:  make(chan *url.URL, 100),
		filesCh: make(chan *url.URL, 100),
		stopCh:  make(chan interface{}),
		client:  http.DefaultClient,
		limiter: limiter,
	}
}

// Run starts crawling from the 'input' URL
func (d *Downloader) Run(input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return errors.New("invalid input URL")
	}

	d.baseURL = u
	d.addURL(u)

	d.wg.Add(1)
	go d.processNewURLs()

	d.wg.Wait()
	return nil
}

var textExtensions = []string{
	".txt",
	".md",
	".css",
	".csv",
	".json",
	".xml",
}

// urlIsTextFile returns true if url points to text file
func urlIsTextFile(u *url.URL) bool {
	for _, extension := range textExtensions {
		if strings.HasSuffix(u.Path, extension) {
			return true
		}
	}

	return false
}
