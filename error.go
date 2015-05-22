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
        "Unable to convert etcd Node into a RR of type %d ('%s'): %s. Node details: %+v",
        e.AttemptedType,
        dns.TypeToString[e.AttemptedType],
        e.Message,
        &e.Node)
}

type RecordValueError struct {
    Message string
    AttemptedType uint16
}
func (e *RecordValueError) Error() string {
    return fmt.Sprintf(
        "Invalid record value for type %d: %s",
        e.AttemptedType,
        e.Message)
}
