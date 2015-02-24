package main

import (
    "github.com/miekg/dns"
    "net"
    "testing"
    "reflect"
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

// Internal utility to save boilerplate. Creates a message with the given
// prereqs and tries to perform an update
func _prereqsTestHelper(t *testing.T, manager *DynamicUpdateManager, prereqMethod string, expected int, prereqs []dns.RR) {

    recordToAdd := &dns.A{
        Hdr: dns.RR_Header{Name: "baz.disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET},
        A: net.ParseIP("1.2.3.4")}

    msg := &dns.Msg{}
    msg.Question = append(msg.Question, dns.Question{Name: "disco.net.", Qclass: dns.ClassINET})
    msg.Insert([]dns.RR{recordToAdd})
    reflPrereqs := reflect.ValueOf(prereqs)
    v := reflect.ValueOf(msg)
    m := v.MethodByName(prereqMethod)
    m.Call([]reflect.Value{reflPrereqs})

    var errorMsg string
    if (expected == dns.RcodeSuccess) {
        errorMsg = "Failed to add DNS record with `" + prereqMethod +"` prereq, got"
    } else {
        errorMsg = "Expected update with `" + prereqMethod +"` prereqs to fail with " + dns.RcodeToString[expected] + ", got"
    }

    result := manager.Update("disco.net.", msg)
    if result.Rcode != expected {
        debugMsg(result)
        t.Error(errorMsg, dns.RcodeToString[result.Rcode])
        t.Fatal()
    }
}

func TestPrerequisites_NameInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameInUse/", true)
    client.Set("TestPrerequisites_NameInUse/net/disco/foo/.A", "1.1.1.1", 0)

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeNameError, []dns.RR{prereq_fail})
    _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeNameError, []dns.RR{prereq_fail, prereq_ok})
    _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeSuccess,   []dns.RR{prereq_ok})
}

func TestPrerequisites_NameNotInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameNotInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameNotInUse/", true)
    client.Set("TestPrerequisites_NameNotInUse/net/disco/foo/.A", "1.1.1.1", 0)

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeYXDomain, []dns.RR{prereq_fail})
    _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeYXDomain, []dns.RR{prereq_fail, prereq_ok})
    _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeSuccess,  []dns.RR{prereq_ok})
}

func TestPrerequisites_ValueIndependentRRSet(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_ValueIndependentRRSet/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_ValueIndependentRRSet/", true)
    client.Set("TestPrerequisites_ValueIndependentRRSet/net/disco/foo/.A", "1.1.1.1", 0)
    client.Set("TestPrerequisites_ValueIndependentRRSet/net/disco/bar/.A", "1.1.1.1", 0)
    client.Set("TestPrerequisites_ValueIndependentRRSet/net/disco/bar/.PTR", "bar.disco.net", 0)

    prereq_foo_a := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET}}
    prereq_foo_ptr := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypePTR, Class: dns.ClassINET}}
    prereq_bar_a := &dns.ANY{ Hdr: dns.RR_Header{Name: "bar.disco.net.", Rrtype: dns.TypeA, Class: dns.ClassINET}}
    prereq_bar_ptr := &dns.ANY{ Hdr: dns.RR_Header{Name: "bar.disco.net.", Rrtype: dns.TypePTR, Class: dns.ClassINET}}

    // same name, many types, expecting failure
    _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeNXRrset, []dns.RR{prereq_foo_a, prereq_foo_ptr})
    // many names, same type, expecting failure
    _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeNXRrset, []dns.RR{prereq_foo_ptr, prereq_bar_ptr})
    // same name, many types, expecting success
    _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeSuccess, []dns.RR{prereq_bar_a, prereq_bar_ptr})
    // many names, same type, expecting success
    _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeSuccess, []dns.RR{prereq_foo_a, prereq_bar_a})
}
