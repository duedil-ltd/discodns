package main

import (
    "github.com/miekg/dns"
    "net"
    "testing"
)

func TestInsertNewRecordNoPrerequsites(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestRecordNoPrerequsites/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    record := &dns.A{
        Hdr: dns.RR_Header{Name: "disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET},
        A: net.ParseIP("1.2.3.4")}

    msg := &dns.Msg{}
    msg.Question = append(msg.Question, dns.Question{Name: "disco.net."})
    msg.Insert([]dns.RR{record})

    result := manager.Update("disco.net.", msg)

    if result.Rcode != dns.RcodeSuccess {
        debugMsg(result)
        t.Error("Failed to insert new DNS record")
        t.Fatal()
    }

    answers, err := resolver.LookupAnswersForType("disco.net.", dns.TypeA)
    if err != nil {
        t.Error("Caught error resolving domain")
        t.Fatal()
    }
    if len(answers) != 1 {
        t.Error("Expected exactly one answer for discodns.net.")
        t.Fatal()
    }
}

func TestDeleteRecordNoPrerequsites(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestRecordNoPrerequsites/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    // record := &dns.A{
    //     Hdr: dns.RR_Header{Name: "disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET},
    //     A: net.ParseIP("1.2.3.4")}

    record := &dns.ANY{Hdr: dns.RR_Header{Name: "disco.net."}}

    msg := &dns.Msg{}
    msg.Question = append(msg.Question, dns.Question{Name: "disco.net."})
    msg.RemoveName([]dns.RR{record})

    result := manager.Update("disco.net.", msg)
    if result.Rcode != dns.RcodeSuccess {
        debugMsg(result)
        t.Error("Failed to remove DNS record")
        t.Fatal()
    }

    answers, err := resolver.LookupAnswersForType("disco.net.", dns.TypeA)
    if err != nil {
        t.Error("Caught error resolving domain")
        t.Fatal()
    }
    if len(answers) > 0 {
        t.Error("Expected zero answers for discodns.net.")
        t.Fatal()
    }
}
