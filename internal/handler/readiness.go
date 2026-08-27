package handler

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type DependencyCheck func(context.Context) error

type Readiness struct {
	database          DependencyCheck
	kubernetesEnabled bool
	migrationsReady   atomic.Bool
	cacheSynced       atomic.Bool
	mu                sync.RWMutex
	dependencies      map[string]DependencyCheck
	timeout           time.Duration
}

func NewReadiness(database DependencyCheck, kubernetesEnabled bool, timeout time.Duration) *Readiness {
	return &Readiness{database: database, kubernetesEnabled: kubernetesEnabled, dependencies: make(map[string]DependencyCheck), timeout: timeout}
}

func (r *Readiness) SetMigrationsReady(ready bool) { r.migrationsReady.Store(ready) }
func (r *Readiness) SetCacheSynced(synced bool)    { r.cacheSynced.Store(synced) }

func (r *Readiness) AddDependency(name string, check DependencyCheck) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dependencies[name] = check
}

func (r *Readiness) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (r *Readiness) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), r.timeout)
	defer cancel()
	checks := map[string]string{}
	ready := true
	if r.database == nil || r.database(ctx) != nil {
		checks["database"] = "unavailable"
		ready = false
	} else {
		checks["database"] = "ready"
	}
	if r.migrationsReady.Load() {
		checks["migrations"] = "ready"
	} else {
		checks["migrations"] = "pending"
		ready = false
	}
	if !r.kubernetesEnabled {
		checks["informerCache"] = "disabled"
	} else if r.cacheSynced.Load() {
		checks["informerCache"] = "ready"
	} else {
		checks["informerCache"] = "not_synced"
		ready = false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, check := range r.dependencies {
		if check == nil || check(ctx) != nil {
			checks[name] = "unavailable"
			ready = false
		} else {
			checks[name] = "ready"
		}
	}
	status := http.StatusOK
	state := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	c.JSON(status, gin.H{"status": state, "checks": checks})
}
