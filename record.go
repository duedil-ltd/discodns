package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "bytes"
    "strings"
    "fmt"
    "strconv"
    "net"
)

type EtcdRecord struct {
    node    *etcd.Node
    ttl     uint32
}

// convertNodeToRR will convert an etcd node with a raw value into a dns.RR
// record, returning an error if the conversion fails
func convertNodeToRR(node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
    rr, err = convertersToRR[header.Rrtype](node, header)
    return
}

// convertRRToNode will convert a DNS RR and it's type specific values to an
// etcd node with a raw value and key path, returning an error if the conversion
// fails
func convertRRToNode(rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    node, err = convertersFromRR[header.Rrtype](rr, header)
    return
}

// nameToKey returns a string representing the etcd version of a domain, replacing dots with slashes
// and reversing it (foo.net. -> /net/foo)
func nameToKey(name string, suffix string) string {
    segments := strings.Split(name, ".")

    var keyBuffer bytes.Buffer
    var writtenSegment bool
    for i := len(segments) - 1; i >= 0; i-- {
        if len(segments[i]) > 0 {
            // We never want to write a leading slash
            if writtenSegment {
                keyBuffer.WriteString("/")
            }

            keyBuffer.WriteString(segments[i])
            writtenSegment = true
        }
    }

    keyBuffer.WriteString(suffix)
    return keyBuffer.String()
}

// Map of conversion functions that turn individual etcd nodes into dns.RR answers
var convertersToRR = map[uint16]func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

    dns.TypeA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

        ip := net.ParseIP(node.Value)
        if ip == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Failed to parse %s as IP Address", node.Value),
                AttemptedType: dns.TypeA,
            }
        } else if ip.To4() == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't an IPv4 address", node.Value),
                AttemptedType: dns.TypeA,
            }
        } else {
            rr = &dns.A{header, ip}
        }

        return
    },

    dns.TypeAAAA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {

        ip := net.ParseIP(node.Value)
        if ip == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Failed to parse IP Address %s", node.Value),
                AttemptedType: dns.TypeAAAA}
        } else if ip.To16() == nil {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't an IPv6 address", node.Value),
                AttemptedType: dns.TypeA}
        } else {
            rr = &dns.AAAA{header, ip}
        }
        return
    },

    dns.TypeTXT: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.TXT{header, []string{node.Value}}
        return
    },

    dns.TypeCNAME: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.CNAME{header, dns.Fqdn(node.Value)}
        return
    },

    dns.TypeNS: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        rr = &dns.NS{header, dns.Fqdn(node.Value)}
        return
    },

    dns.TypePTR: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        labels, ok := dns.IsDomainName(node.Value)

        if (ok && labels > 0) {
            rr = &dns.PTR{header, dns.Fqdn(node.Value)}
        } else {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value '%s' isn't a valid domain name", node.Value),
                AttemptedType: dns.TypePTR}
        }
        return
    },

    dns.TypeSRV: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        parts := strings.SplitN(node.Value, "\t", 4)

        if len(parts) != 4 {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't valid for SRV", node.Value),
                AttemptedType: dns.TypeSRV}
        } else {

            priority, err := strconv.ParseUint(parts[0], 10, 16)
            if err != nil {
                return nil, err
            }

            weight, err := strconv.ParseUint(parts[1], 10, 16)
            if err != nil {
                return nil, err
            }

            port, err := strconv.ParseUint(parts[2], 10, 16)
            if err != nil {
                return nil, err
            }

            target := dns.Fqdn(parts[3])

            rr = &dns.SRV{
                header,
                uint16(priority),
                uint16(weight),
                uint16(port),
                target}
        }
        return
    },

    dns.TypeSOA: func (node *etcd.Node, header dns.RR_Header) (rr dns.RR, err error) {
        parts := strings.SplitN(node.Value, "\t", 6)

        if len(parts) < 6 {
            err = &NodeConversionError{
                Node: node,
                Message: fmt.Sprintf("Value %s isn't valid for SOA", node.Value),
                AttemptedType: dns.TypeSOA}
        } else {
            refresh, err := strconv.ParseUint(parts[2], 10, 32)
            if err != nil {
                return nil, err
            }

            retry, err := strconv.ParseUint(parts[3], 10, 32)
            if err != nil {
                return nil, err
            }

            expire, err := strconv.ParseUint(parts[4], 10, 32)
            if err != nil {
                return nil, err
            }

            minttl, err := strconv.ParseUint(parts[5], 10, 32)
            if err != nil {
                return nil, err
            }

            rr = &dns.SOA{
                Hdr:     header,
                Ns:      dns.Fqdn(parts[0]),
                Mbox:    dns.Fqdn(parts[1]),
                Refresh: uint32(refresh),
                Retry:   uint32(retry),
                Expire:  uint32(expire),
                Minttl:  uint32(minttl)}
        }

        return
    },
}

// Map of conversion functions that turn dns.RR answers into individual etcd nodes
var convertersFromRR = map[uint16]func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {

    dns.TypeANY: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
        node = &etcd.Node{Key: nameToKey(header.Name, "")}

        return
    },

    dns.TypeA: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
        if record, ok := rr.(*dns.A); ok {
            node = &etcd.Node{
                Key: nameToKey(header.Name, "/.A"),
                Value: record.A.String()}
        }

        return
    },

    // dns.TypeAAAA: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypeTXT: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypeCNAME: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypeNS: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypePTR: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypeSRV: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },

    // dns.TypeSOA: func (rr dns.RR, header dns.RR_Header) (node *etcd.Node, err error) {
    //     panic("Not implemented")
    // },
}
