package app

import "sync"

type urlsMap struct {
	m map[string]bool
	l sync.RWMutex
}

