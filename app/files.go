package app

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"time"
)

// processNewFiles reads from filesCh
// and downloads them
func (d *Downloader) processNewFiles() {
	defer d.wg.Done()

	// when stopCh is closed
	// processNewFile goroutines will read all urls from channel to map
	// and exit, so we don't need to select from stopCh here.
	for u := range d.filesCh {
		go d.processNewFile(u)
	}
}

// addURL should be used instead direct write to channel
// in order to ube able to manage processNewURL goroutines
func (d *Downloader) addFile(u *url.URL) {
	d.filesWG.Add(1)
	d.filesCh <- u
}

func (d *Downloader) processNewFile(u *url.URL) {
	input := u.String()

	defer d.filesWG.Done()

	d.filesLock.Lock()

	// check if url was not processed
	if _, ok := d.files[input]; ok {
		d.filesLock.Unlock()
		return
	}

	d.files[input] = false
	d.filesLock.Unlock()

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
		case <-d.stopCh:
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
