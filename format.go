package main

import (
    "bytes"
    "strings"
)

// nameToKey returns a string representing the etcd version of a domain, replacing dots with slashes
// and reversing it (foo.net. -> /net/foo)
func nameToKey(name string, suffix string) string {
    segments := strings.Split(name, ".")

    var keyBuffer bytes.Buffer
    for i := len(segments) - 1; i >= 0; i-- {
        if len(segments[i]) > 0 {
            keyBuffer.WriteString("/")
            keyBuffer.WriteString(segments[i])
        }
    }

    keyBuffer.WriteString(suffix)
    return keyBuffer.String()
}
