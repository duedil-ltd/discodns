package main

import (
    "testing"
)

func TestLock(t *testing.T) {
    // Enable debug logging
    log_debug = true
}

func TestSimpleLockUnlock(t *testing.T) {
    client.Delete("TestSimpleLockUnlock/", true)

    lock := lockDomain(client, "discodns.net", "TestSimpleLockUnlock")

    if lock.IsLocked() != true {
        t.Error("Expected lock to be locked, it was not")
        t.Fatal()
    }

    lock.Unlock(false)

    if lock.IsLocked() != false {
        t.Error("Expected lock to be unlocked, it was not")
        t.Fatal()
    }

}

func TestConflictingLock(t *testing.T) {
    client.Delete("TestConflictingLock/", true)

    lockA := lockDomain(client, "discodns.net", "TestConflictingLock/")
    if lockA.IsLocked() != true {
        t.Error("Expected lock to be locked, it was not")
        t.Fatal()
    }

    // Defer a function to handle the panic when we request overlapping
    // locks. We do this here because if the lock fails to be acquired above
    // the test should fail exceptionally.
    defer func() {
        p := recover()
        if p == nil {
            t.Error("Expected a panic, got nil")
            t.Fatal()
        }
    }()

    lockDomain(client, "discodns.net", "TestConflictingLock/")
}
