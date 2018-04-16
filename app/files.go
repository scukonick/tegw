package app

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"sync"
	"time"
)

// processNewFiles reads from filesCh
// and downloads them
func (d *Downloader) processNewFilesV2(ctx context.Context, filesCh chan *url.URL) {
	wg := &sync.WaitGroup{}

	for u := range filesCh {
		wg.Add(1)
		go func() {
			d.processNewFile(ctx, u)
			wg.Done()
		}()
	}

	wg.Wait()
}

func (d *Downloader) addFile(ctx context.Context, u *url.URL, filesCh chan *url.URL) {
	input := u.String()

	d.filesLock.Lock()

	// check if url was not processed
	if _, ok := d.files[input]; ok {
		d.filesLock.Unlock()
		return
	}

	d.files[input] = false
	d.filesLock.Unlock()

	select {
	case filesCh <- u:
	case <-ctx.Done():
	}
}

func (d *Downloader) processNewFile(ctx context.Context, u *url.URL) {
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
		log.Printf("ERR: failed to download file %s: %v", input, err)
		return
	}
	defer closeC(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("ERR: invalid code %s: %d", input, resp.StatusCode)
		return
	}

	urlPath := resp.Request.URL.Path

	_, filename := path.Split(urlPath)
	hash := hashURL(resp.Request.URL.String()) // to prevent check of unique filenames
	fullPath := path.Join(d.outDir, hash+"_"+filename)
	tmpPath := fullPath + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("ERR: failed to open file: %v", err)
		return
	}

downloadLoop:
	for {
		select {
		case <-ctx.Done():
			closeC(f)
			cleanTmp(tmpPath)
			return
		default:
			_, err := io.CopyN(f, resp.Body, 64*1024)
			if err != nil && err != io.EOF {
				log.Printf("ERR: download failed: %v", err)
				closeC(f)
				cleanTmp(tmpPath)
				return
			}
			if err == io.EOF {
				break downloadLoop
			}
		}
	}

	err = f.Close()
	if err != nil {
		log.Printf("ERR: failed to close file: %v", err)
		cleanTmp(tmpPath)
		return
	}

	err = os.Rename(tmpPath, fullPath)
	if err != nil {
		log.Printf("failed to rename %s -> %s: %v", tmpPath, fullPath, err)
		return
	}

	d.filesLock.Lock()
	d.files[input] = true
	d.filesLock.Unlock()
}

func hashURL(link string) string {
	s := md5.Sum([]byte(link))

	return hex.EncodeToString(s[:])
}

func cleanTmp(fullPath string) {
	err := os.Remove(fullPath)
	if err != nil {
		log.Printf("failed to remove tmp file %s: %v", fullPath, err)
	}
}
