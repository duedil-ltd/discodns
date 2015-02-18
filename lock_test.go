package main

import (
    "testing"
    "time"
)

func TestSimpleLockUnlock(t *testing.T) {
    testKey := "TestSimpleLockUnlock/.lock"
    client.Delete(testKey, true)

    lock := NewEtcdKeyLock(client, testKey)
    locked, err := lock.WaitForAcquire(1)

    if !locked || err != nil {
        t.Error("Expected to acquire lock, failed")
        t.Fatal()
    }
    _, err = client.Get(testKey, false, true)
    if err != nil {
        t.Error("Lock claimed to succeed but etcd record missing/broken")
        t.Fatal()
    }

    lock.Abandon()
    time.Sleep(500 * time.Millisecond)
    _, err = client.Get(testKey, false, true)
    if err == nil {
        t.Error("Lock abandoned, but key exists")
        t.Fatal()
    }
}
