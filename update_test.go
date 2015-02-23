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

func TestPrerequisites_NameInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameInUse/", true)
    client.Set("TestPrerequisites_NameInUse/net/disco/foo/.A", "1.1.1.1", 0)

    recordToAdd := &dns.A{
        Hdr: dns.RR_Header{Name: "bar.disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET},
        A: net.ParseIP("1.2.3.4")}

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    msg_1 := &dns.Msg{}
    msg_1.Question = append(msg_1.Question, dns.Question{Name: "bar.disco.net."})
    msg_1.Insert([]dns.RR{recordToAdd})
    msg_1.NameUsed([]dns.RR{prereq_fail})

    result_1 := manager.Update("disco.net.", msg_1)
    if result_1.Rcode != dns.RcodeNameError {
        debugMsg(result_1)
        t.Error("expected update to fail with NXDOMAIN, actually got", dns.RcodeToString[result_1.Rcode])
        t.Fatal()
    }

    msg_2 := &dns.Msg{}
    msg_2.Question = append(msg_2.Question, dns.Question{Name: "bar.disco.net."})
    msg_2.Insert([]dns.RR{recordToAdd})
    msg_2.NameUsed([]dns.RR{prereq_fail, prereq_ok})

    result_2 := manager.Update("disco.net.", msg_2)
    if result_2.Rcode != dns.RcodeNameError {
        debugMsg(result_2)
        t.Error("expected update to fail with NXDOMAIN, actually got", dns.RcodeToString[result_2.Rcode])
        t.Fatal()
    }

    msg_3 := &dns.Msg{}
    msg_3.Question = append(msg_3.Question, dns.Question{Name: "bar.disco.net."})
    msg_3.Insert([]dns.RR{recordToAdd})
    msg_3.NameUsed([]dns.RR{prereq_ok})

    result_3 := manager.Update("disco.net.", msg_3)
    if result_3.Rcode != dns.RcodeSuccess {
        debugMsg(result_3)
        t.Error("Failed to add DNS record with name-in-use prereq, got", dns.RcodeToString[result_3.Rcode])
        t.Fatal()
    }
}

func TestPrerequisites_NameNotInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameNotInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameNotInUse/", true)
    client.Set("TestPrerequisites_NameNotInUse/net/disco/foo/.A", "1.1.1.1", 0)

    recordToAdd := &dns.A{
        Hdr: dns.RR_Header{Name: "bar.disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET},
        A: net.ParseIP("1.2.3.4")}

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    msg_1 := &dns.Msg{}
    msg_1.Question = append(msg_1.Question, dns.Question{Name: "bar.disco.net."})
    msg_1.Insert([]dns.RR{recordToAdd})
    msg_1.NameNotUsed([]dns.RR{prereq_fail})

    result_1 := manager.Update("disco.net.", msg_1)
    if result_1.Rcode != dns.RcodeYXDomain {
        debugMsg(result_1)
        t.Error("expected update to fail with RcodeYXDomain, actually got", dns.RcodeToString[result_1.Rcode])
        t.Fatal()
    }

    msg_2 := &dns.Msg{}
    msg_2.Question = append(msg_2.Question, dns.Question{Name: "bar.disco.net."})
    msg_2.Insert([]dns.RR{recordToAdd})
    msg_2.NameNotUsed([]dns.RR{prereq_fail, prereq_ok})

    result_2 := manager.Update("disco.net.", msg_2)
    if result_2.Rcode != dns.RcodeYXDomain {
        debugMsg(result_2)
        t.Error("expected update to fail with RcodeYXDomain, actually got", dns.RcodeToString[result_2.Rcode])
        t.Fatal()
    }

    msg_3 := &dns.Msg{}
    msg_3.Question = append(msg_3.Question, dns.Question{Name: "bar.disco.net."})
    msg_3.Insert([]dns.RR{recordToAdd})
    msg_3.NameNotUsed([]dns.RR{prereq_ok})

    result_3 := manager.Update("disco.net.", msg_3)
    if result_3.Rcode != dns.RcodeSuccess {
        debugMsg(result_3)
        t.Error("Failed to add DNS record with name-not-in-use prereq, got", dns.RcodeToString[result_3.Rcode])
        t.Fatal()
    }
}
