package app

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
)

// Downloader is a crawler which parses incoming url
// and stores text files to disk
type Downloader struct {
	urls          map[string]bool
	files         map[string]bool
	restoredURLs  []*url.URL
	restoredFiles []*url.URL
	urlsCh        chan *url.URL
	urlsLock      sync.RWMutex
	filesLock     sync.RWMutex
	client        *http.Client
	limiter       chan interface{} // limits number of simultaneous downloads
	baseURL       *url.URL
	urlsWG        sync.WaitGroup
	wg            sync.WaitGroup
	stateFile     string
	outDir        string
	cancel        context.CancelFunc
	ctx           context.Context
	timeout       time.Duration
}

func NewDownloader(outDir, stateDir string) *Downloader {
	limiter := make(chan interface{}, 10)
	for i := 0; i < 10; i++ {
		limiter <- true
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Downloader{
		urls:          make(map[string]bool, 100),
		files:         make(map[string]bool, 100),
		restoredURLs:  make([]*url.URL, 0, 100),
		restoredFiles: make([]*url.URL, 0, 100),
		urlsCh:        make(chan *url.URL),
		client:        http.DefaultClient,
		limiter:       limiter,
		outDir:        outDir,
		stateFile:     path.Join(stateDir, "state.yaml"),
		ctx:           ctx,
		cancel:        cancel,
		timeout:       10 * time.Second,
	}
}

// Run starts crawling from the 'input' URL
func (d *Downloader) Run(input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return errors.New("invalid input URL")
	}

	d.baseURL = u

	err = d.loadState()
	if err == errNoState {
		d.addURL(d.ctx, u)
	} else if err != nil {
		log.Printf("ERR: failed to load state: %v", err)
		return err
	} else {
		// starting download of old urls
		for _, u := range d.restoredURLs {
			d.addURL(d.ctx, u)
		}

	}

	c := d.processNewURLsV2(d.ctx)
	d.processNewFilesV2(d.ctx, c)

	log.Print("saving state...")
	d.saveState()
	log.Print("state saved")
	return nil
}

func (d *Downloader) Stop() {
	d.cancel()
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
