package utils

import "sync"

// Wrap function in write/default lock from the mutex
func WithLock(mtx *sync.RWMutex, callback func()) {
	defer mtx.Unlock()
	mtx.Lock()
	callback()
}

// Wrap function in read mode lock from the mutex
func WithRLock(mtx *sync.RWMutex, callback func()) {
	defer mtx.RUnlock()
	mtx.RLock()
	callback()
}
