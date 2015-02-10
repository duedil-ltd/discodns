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

func TestInsertMultipleRecords(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestInsertMultipleRecords/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix
    client.Delete("TestInsertMultipleRecords/", true)

    record1 := &dns.SRV{
        Hdr: dns.RR_Header{Name: "disco.net.", Rrtype: dns.TypeSRV, Class: dns.ClassINET},
        Port: 80, Priority: 100, Weight: 100, Target: "foo.disco.net"}

    record2 := &dns.TXT{
        Hdr: dns.RR_Header{Name: "disco.net.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
        Txt: []string{"lol"}}

    msg := &dns.Msg{}
    msg.Question = append(msg.Question, dns.Question{Name: "disco.net."})
    msg.Insert([]dns.RR{record1, record2})

    result := manager.Update("disco.net.", msg)

    if result.Rcode != dns.RcodeSuccess {
        debugMsg(result)
        t.Error("Failed to add DNS records")
        t.Fatal()
    }

    srvAnswers, err := resolver.LookupAnswersForType("disco.net.", dns.TypeSRV)
    if err != nil {
        t.Error("Caught error retrieving SRV")
        t.Fatal()
    }
    if len(srvAnswers) != 1 {
        t.Error("Expected one SRV response")
        t.Fatal()
    }
    txtAnswers, err := resolver.LookupAnswersForType("disco.net.", dns.TypeTXT)
    if err != nil {
        t.Error("Caught error retrieving txt")
        t.Fatal()
    }
    if len(txtAnswers) != 1 {
        t.Error("Expected one TXT response")
        t.Fatal()
    }
}
