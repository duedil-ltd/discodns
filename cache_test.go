package main

import (
    "github.com/rcrowley/go-metrics"
    "testing"
    "time"
)

var (
    cache = MakeEtcdRecordCache(metrics.GetOrRegisterCounter("cache.hit", metrics.DefaultRegistry),
                                metrics.GetOrRegisterCounter("cache.miss", metrics.DefaultRegistry))
)

func TestCache(t *testing.T) {
    log_debug = true
}

func TestGetMiss(t *testing.T) {
    _, hit := cache.Get("cache.miss")
    if hit != false {
        t.Error("Expected cache miss")
        t.Fatal()
    }
}

func TestCacheHit(t *testing.T) {
    nodes := make([]*EtcdRecord, 2)
    cache.Set("cache.hit", nodes, time.Second * 5)

    _, hit := cache.Get("cache.hit")
    if hit != true {
        t.Error("Expected cache hit")
        t.Fatal()
    }
}

func TestCacheMissExpiry(t *testing.T) {
    nodes := make([]*EtcdRecord, 2)
    cache.Set("cache.expired", nodes, time.Second * 1)

    <- time.After(time.Second * 2)

    _, hit := cache.Get("cache.expired")
    if hit != false {
        t.Error("Expected cache miss")
        t.Fatal()
    }
}
