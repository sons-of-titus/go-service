package stats

import (
	"github.com/sons-of-titus/go-service/models"
	"sync"
)

type result struct {
	latest models.Statistics
	lock   sync.Mutex
}

type Result interface {
	Get() models.Statistics
	Combine(stats models.Statistics)
}

// Get returns the save statistics result
func (r *result) Get() models.Statistics {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.latest
}

// Combine updates the result statistics
func (r *result) Combine(stats models.Statistics) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.latest = models.Combine(r.latest, stats)
}
