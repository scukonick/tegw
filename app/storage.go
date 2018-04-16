package app

import (
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"errors"

	"gopkg.in/yaml.v2"
)

var errNoState = errors.New("no state fie")

type state struct {
	URLs  map[string]bool
	Files map[string]bool
}

func (d *Downloader) saveState() {
	f, err := os.OpenFile(d.stateFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("ERR: failed to save state: %v", err)
		return
	}
	defer closeC(f)

	s := &state{
		URLs:  make(map[string]bool, 100),
		Files: make(map[string]bool, 100),
	}

	d.urlsLock.RLock()
	for link, downloaded := range d.urls {
		s.URLs[link] = downloaded
	}
	d.urlsLock.RUnlock()

	d.filesLock.RLock()
	for file, downloaded := range d.files {
		s.Files[file] = downloaded
	}
	d.filesLock.RUnlock()

	data, err := yaml.Marshal(s)
	if err != nil {
		log.Printf("failed to marhsal state: %+v", err)
		return
	}

	_, err = f.Write(data)
	if err != nil {
		log.Printf("failed to write state: %+v", err)
		return
	}

	err = f.Sync()
	if err != nil {
		log.Printf("failed to sync statefile: %+v", err)
		return
	}

}

func (d *Downloader) loadState() error {
	f, err := os.Open(d.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// not loading, just running as is
			return errNoState
		}
		return err
	}
	defer closeC(f)

	s := &state{
		URLs:  make(map[string]bool, 100),
		Files: make(map[string]bool, 100),
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, s)
	if err != nil {
		return err
	}

	for link, processed := range s.URLs {
		if processed {
			d.urls[link] = processed
			continue
		}

		u, err := url.Parse(link)
		if err != nil {
			log.Printf("WARN: failed to parse stored url")
			continue
		}

		d.restoredURLs = append(d.restoredURLs, u)
	}

	for link, processed := range s.Files {
		if processed {
			d.files[link] = processed
			continue
		}

		u, err := url.Parse(link)
		if err != nil {
			log.Printf("WARN: failed to parse stored file")
			continue
		}

		d.restoredFiles = append(d.restoredFiles, u)
	}

	return nil
}
