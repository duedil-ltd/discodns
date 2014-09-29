package main

import (
    "testing"
)

func TestRecord(t *testing.T) {
    // Enable debug logging
    log_debug = true
}

func TestNameToKey(t *testing.T) {

    key := nameToKey("foo.disco.net", "")
    if key != "net/disco/foo" {
        t.Error("Expected key to be /net/disco/foo but got " + key)
        t.Fatal()
    }
}

func TestNameToKeyFQDN(t *testing.T) {

    key := nameToKey("foo.disco.net.", "")
    if key != "net/disco/foo" {
        t.Error("Expected key to be /net/disco/foo but got " + key)
        t.Fatal()
    }
}

func TestNameToKeyWithSuffix(t *testing.T) {

    key := nameToKey("foo.disco.net", "/.A")
    if key != "net/disco/foo/.A" {
        t.Error("Expected key to be /net/disco/foo/.A but got " + key)
        t.Fatal()
    }
}

func TestNameToKeyFQDNWithSuffix(t *testing.T) {

    key := nameToKey("foo.disco.net.", "/.A")
    if key != "net/disco/foo/.A" {
        t.Error("Expected key to be /net/disco/foo/.A but got " + key)
        t.Fatal()
    }
}
