package main

import (
    "github.com/rcrowley/go-metrics"
    "github.com/nu7hatch/gouuid"
    "sync"
    "time"
)

type EtcdRecordCache struct {
    hitCounter      metrics.Counter
    missCounter     metrics.Counter

    cache struct{ sync.RWMutex
        m map[string][]*EtcdRecord
    }
    values struct{ sync.RWMutex
        m map[string]string
    }
}

func MakeEtcdRecordCache(hitCounter, missCounter metrics.Counter) EtcdRecordCache {
    cache := EtcdRecordCache{
        hitCounter: hitCounter,
        missCounter: missCounter}

    cache.cache.m = make(map[string][]*EtcdRecord)
    cache.values.m = make(map[string]string)

    return cache
}

// Get will return a slice of `nodes` stored under the given key in the cache. If
// the query is a cache hit, the return value of `hit` will be true.
func (c *EtcdRecordCache) Get(key string) (nodes []*EtcdRecord, hit bool) {
    hit = false

    debugMsg("Cache Get: " + key)

    c.values.RLock()
    pointer, ok := c.values.m[key]
    c.values.RUnlock()

    if ok {
        debugMsg("Cache Hit")
        debugMsg("Cache Pointer: " + pointer)

        // Grab the value from the value cache
        c.cache.RLock()
        nodes = c.cache.m[pointer]
        c.cache.RUnlock()

        hit = true
        c.hitCounter.Inc(1)
    } else {
        debugMsg("Cache Miss")
        c.missCounter.Inc(1)
    }

    return
}

// Set will insert a new entry into the cache, storing the set of nodes in memory
// against the key. After the given TTL has expired, the item will be removed
// from the cache.
func (c *EtcdRecordCache) Set(key string, nodes []*EtcdRecord, ttl time.Duration) {

    // TODO(tarnfeld): Error handling
    pointer_uuid, _ := uuid.NewV4()
    pointer := pointer_uuid.String()

    // Insert the value
    c.cache.Lock()
    c.cache.m[pointer] = nodes
    c.cache.Unlock()

    // Replace the pointer
    c.values.Lock()
    c.values.m[key] = pointer
    c.values.Unlock()

    // Spawn a goroutine to expire after the TTL
    go func() {
        <- time.After(ttl)

        debugMsg("Cache Expiry: " + key)

        c.values.Lock()
        if c.values.m[key] == pointer {
            delete(c.values.m, key)
        }
        c.values.Unlock()

        debugMsg("Cache Cleanup: " + pointer)

        c.cache.Lock()
        delete(c.cache.m, pointer)
        c.cache.Unlock()
    }()

    return
}
