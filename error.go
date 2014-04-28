package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "fmt"
)

type NodeConversionError struct {
    Message string
    Node *etcd.Node
    AttemptedType uint16
}

func (e *NodeConversionError) Error() string {
    return fmt.Sprintf(
        "Unable to convert etc Node into a RR of type %d ('%s'): %s. Node details: %+v",
        e.AttemptedType,
        dns.TypeToString[e.AttemptedType],
        e.Message,
        &e.Node)
}
