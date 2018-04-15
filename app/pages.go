package app

import (
	"errors"
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// processNewURLs reads from urlsCh,
// downloads them, parses and sends found urls
// and files to urlsCh and filesCh
func (d *Downloader) processNewURLs() {
	defer d.wg.Done()

	noMoreURLsCH := make(chan interface{})

	go func() {
		// close noMoreUrlsCh if there are no
		// goroutines parsing pages
		d.urlsWG.Wait()
		close(noMoreURLsCH)
	}()

	// when stopCh is closed
	// processNewURL goroutines will read all urls from channel to map
	// and exit, so we don't need to select from stopCh here.
	for {
		select {
		case u := <-d.urlsCh:
			go d.processNewURL(u)
		case <-noMoreURLsCH:
			return
		}
	}

}

// addURL should be used instead direct write to channel
// in order to ube able to manage processNewURL goroutines
func (d *Downloader) addURL(u *url.URL) {
	d.urlsWG.Add(1)
	d.urlsCh <- u
}

func (d *Downloader) processNewURL(u *url.URL) {
	input := u.String()

	defer d.urlsWG.Done()

	d.urlsLock.Lock()

	// check if url was not processed
	if _, ok := d.urls[input]; ok {
		d.urlsLock.Unlock()
		return
	}

	d.urls[input] = false
	d.urlsLock.Unlock()

	select {
	case <-d.stopCh:
		return
	default:
		// Because order of select is not guaranteed
		// checking only stop channel here.
	}

	select {
	case <-d.stopCh:
		return
	case <-d.limiter:
	}

	log.Printf("GET %s", input)
	now := time.Now()
	resp, err := d.client.Get(input)
	log.Printf("Done %s, took: %v", input, time.Since(now))

	d.limiter <- true
	if err != nil {
		log.Printf("ERR: failed to download url %s: %v", input, err)
		return
	}
	defer closeC(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("ERR: failed to download url %s: http %d", input, resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		log.Printf("ERR: failed to download url %s: invalid content type: %s",
			input, contentType)
		return
	}

	urls, files, err := d.parseResp(resp.Body)
	if err != nil {
		log.Printf("ERR: failed to download url %s: %v", input, err)
		return
	}

	// using resp.RequestURL to handle relative URLs after redirects
	urls = d.filterURLs(resp.Request.URL, urls)
	for _, v := range urls {
		d.addURL(v)
	}

	for _, v := range files {
		// not checking files url domain, only replace relative urls
		d.filesCh <- u.ResolveReference(v)
	}

	d.urlsLock.Lock()
	d.urls[input] = true
	d.urlsLock.Unlock()
}

// filterURLs removes urls which are not 'sub-urls' for our base URL
// and also replaces relative urls with absolute ones
func (d *Downloader) filterURLs(referer *url.URL, urls []*url.URL) []*url.URL {
	filteredURLs := make([]*url.URL, 0, len(urls))
	for _, u := range urls {
		if !u.IsAbs() {
			filteredURLs = append(filteredURLs, referer.ResolveReference(u))
			continue
		}

		err := d.checkURL(u)
		if err != nil {
			continue
		}
		filteredURLs = append(filteredURLs, u)
	}

	return filteredURLs
}

// checkURL checks if input URL has the same domain
// as baseURL and it's path contains path of baseURL.
// It does not check if the scheme is different.
func (d *Downloader) checkURL(u *url.URL) error {
	if d.baseURL.Host != u.Host {
		return errors.New("invalid host")
	}

	basePath := d.baseURL.Path
	newPath := u.Path

	if basePath == newPath {
		return nil
	}

	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	if !strings.HasPrefix(newPath, basePath) {
		return errors.New("invalid path")
	}

	return nil
}

// parseResp parses response body and returns
// slice of new urls and new file urls.
func (d *Downloader) parseResp(i io.Reader) ([]*url.URL, []*url.URL, error) {
	urls := make([]*url.URL, 0, 100)
	files := make([]*url.URL, 0, 100)

	doc, err := goquery.NewDocumentFromReader(i)
	if err != nil {
		return nil, nil, err
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			// ignoring a without href
			return
		}

		u, err := url.Parse(href)
		if err != nil {
			// ignoring invalid url
			return
		}

		if urlIsTextFile(u) {
			files = append(files, u)
			return
		}

		urls = append(urls, u)
	})

	return urls, files, err
}
