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
func _prereqsTestHelper(t *testing.T, manager *DynamicUpdateManager, prereqMethod string, expected int, prereqs []dns.RR) (pass bool) {

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
        return false
    }
    return true
}

func TestPrerequisites_NameInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameInUse/", true)
    client.Set("TestPrerequisites_NameInUse/net/disco/foo/.A", "1.1.1.1", 0)

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    if ! _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeNameError, []dns.RR{prereq_fail}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeNameError, []dns.RR{prereq_fail, prereq_ok}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "NameUsed", dns.RcodeSuccess,   []dns.RR{prereq_ok}) { t.Fatal() }
}

func TestPrerequisites_NameNotInUse(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_NameNotInUse/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_NameNotInUse/", true)
    client.Set("TestPrerequisites_NameNotInUse/net/disco/foo/.A", "1.1.1.1", 0)

    prereq_fail := &dns.ANY{ Hdr: dns.RR_Header{Name: "foo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}
    prereq_ok := &dns.ANY{ Hdr: dns.RR_Header{Name: "foofoo.disco.net.", Rrtype: dns.TypeANY, Class: dns.ClassINET}}

    if ! _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeYXDomain, []dns.RR{prereq_fail}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeYXDomain, []dns.RR{prereq_fail, prereq_ok}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "NameNotUsed", dns.RcodeSuccess,  []dns.RR{prereq_ok}) { t.Fatal() }
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

    if ! _prereqsTestHelper(t, manager, "RRsetNotUsed", dns.RcodeSuccess, []dns.RR{prereq_foo_ptr}) { t.Fatal() }

    if ! _prereqsTestHelper(t, manager, "RRsetUsed",    dns.RcodeNXRrset, []dns.RR{prereq_foo_a, prereq_foo_ptr}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "RRsetNotUsed", dns.RcodeYXRrset, []dns.RR{prereq_foo_a, prereq_foo_ptr}) { t.Fatal() }

    if ! _prereqsTestHelper(t, manager, "RRsetUsed",    dns.RcodeNXRrset, []dns.RR{prereq_foo_ptr, prereq_bar_ptr}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "RRsetNotUsed", dns.RcodeYXRrset, []dns.RR{prereq_foo_ptr, prereq_bar_ptr}) { t.Fatal() }

    if ! _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeSuccess, []dns.RR{prereq_bar_a, prereq_bar_ptr}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "RRsetNotUsed", dns.RcodeYXRrset, []dns.RR{prereq_bar_a, prereq_bar_ptr}) { t.Fatal() }

    if ! _prereqsTestHelper(t, manager, "RRsetUsed", dns.RcodeSuccess, []dns.RR{prereq_foo_a, prereq_bar_a}) { t.Fatal() }
}

func TestPrerequisites_ValueDependentRRSet(t *testing.T) {
    manager := &DynamicUpdateManager{etcd: client, etcdPrefix: "TestPrerequisites_ValueDependentRRSet/", resolver: resolver}
    resolver.etcdPrefix = manager.etcdPrefix

    client.Delete("TestPrerequisites_ValueDependentRRSet/", true)
    client.Set("TestPrerequisites_ValueDependentRRSet/net/disco/foo/.A", "1.1.1.1", 0)
    client.Set("TestPrerequisites_ValueDependentRRSet/net/disco/bar/.A", "1.1.1.1", 0)
    client.Set("TestPrerequisites_ValueDependentRRSet/net/disco/bar/.PTR", "match.disco.net", 0)

    prereq_foo_a_match, _   := dns.NewRR("foo.disco.net. 0 IN A 1.1.1.1")
    prereq_foo_a_miss, _    := dns.NewRR("foo.disco.net. 0 IN A 2.2.2.2")
    prereq_bar_a_match, _   := dns.NewRR("bar.disco.net. 0 IN A 1.1.1.1")
    prereq_bar_a_miss, _    := dns.NewRR("bar.disco.net. 0 IN A 2.2.2.2")
    prereq_bar_ptr_match, _ := dns.NewRR("bar.disco.net. 0 IN PTR match.disco.net")
    prereq_bar_ptr_miss, _  := dns.NewRR("bar.disco.net. 0 IN PTR miss.disco.net")

    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeNXRrset, []dns.RR{prereq_foo_a_miss}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeNXRrset, []dns.RR{prereq_foo_a_miss, prereq_bar_a_miss}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeNXRrset, []dns.RR{prereq_foo_a_miss, prereq_bar_ptr_miss}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeNXRrset, []dns.RR{prereq_foo_a_match, prereq_bar_a_miss}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeNXRrset, []dns.RR{prereq_foo_a_match, prereq_bar_ptr_miss}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeSuccess, []dns.RR{prereq_foo_a_match, prereq_bar_a_match}) { t.Fatal() }
    if ! _prereqsTestHelper(t, manager, "Used", dns.RcodeSuccess, []dns.RR{prereq_foo_a_match, prereq_bar_a_match, prereq_bar_ptr_match}) { t.Fatal() }
}
