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

func TestConflictingLock(t *testing.T) {
    testKey := "TestConflictingLock/.lock"
    client.Delete(testKey, true)

    lock_a := NewEtcdKeyLock(client, testKey)
    lock_a.WaitForAcquire(30)

    lock_b := NewEtcdKeyLock(client, testKey)
    b_locked, b_err := lock_b.WaitForAcquire(1)
    if b_locked || b_err == nil {
        t.Error("Expected second lock to timeout")
        t.Fatal()
    }

    lock_c := NewEtcdKeyLock(client, testKey)
    go func(){
        time.Sleep(500 * time.Millisecond)
        lock_a.Abandon()
    }()

    c_locked, c_err := lock_c.WaitForAcquire(5)
    if !c_locked || c_err != nil {
        t.Error("Expected third lock to succeed in time")
        t.Fatal()
    }
}
