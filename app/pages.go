package app

import (
	"errors"
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/context"
)

func (d *Downloader) processNewURLsV2(ctx context.Context) chan *url.URL {
	c := make(chan interface{})

	filesCh := make(chan *url.URL)

	for _, u := range d.restoredFiles {
		d.urlsWG.Add(1)
		go func() {
			d.addFile(ctx, u, filesCh)
			d.urlsWG.Done()
		}()
	}

	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			case u := <-d.urlsCh:
				go func() {
					d.processNewURLV2(ctx, u, filesCh)
					d.urlsWG.Done()
				}()
			case <-c:
				return
			}
		}
	}()

	go func() {
		d.urlsWG.Wait()
		close(c)
		close(filesCh)
	}()

	return filesCh
}

// addURL should be used instead direct write to channel
// in order to ube able to manage processNewURL goroutines
func (d *Downloader) addURL(ctx context.Context, u *url.URL) {
	input := u.String()

	d.urlsLock.Lock()

	// check if url was not processed
	if _, ok := d.urls[input]; ok {
		d.urlsLock.Unlock()
		return
	}

	d.urls[input] = false
	d.urlsLock.Unlock()

	d.urlsWG.Add(1)

	go func() {
		select {
		case d.urlsCh <- u:
		case <-ctx.Done():
			d.urlsWG.Done()
		}
	}()
}

func (d *Downloader) processNewURLV2(ctx context.Context, u *url.URL, filesCh chan *url.URL) {
	input := u.String()

	select {
	case <-ctx.Done():
		return
	case <-d.limiter:
	}

	log.Printf("GET %s", input)
	now := time.Now()
	req := d.buildRequest(ctx, input)
	resp, err := d.client.Do(req)
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
		d.addURL(ctx, v)
	}

	for _, v := range files {
		// not checking files url domain, only replace relative urls
		d.addFile(ctx, resp.Request.URL.ResolveReference(v), filesCh)
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
