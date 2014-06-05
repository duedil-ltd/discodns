package main

import (
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "testing"
)

var (
    client = etcd.NewClient([]string{"127.0.0.1:4001"})
    resolver = &Resolver{etcd: client}
)

func TestEtcd(t *testing.T) {
    // Enable debug logging
    log_debug = true

    if !client.SyncCluster() {
        t.Error("Failed to sync etcd cluster")
        t.Fatal()
    }
}

func TestGetFromStorageSingleKey(t *testing.T) {
    resolver.etcdPrefix = "TestGetFromStorageSingleKey/"
    client.Set("TestGetFromStorageSingleKey/net/disco/.A", "1.1.1.1", 0)

    nodes, err := resolver.GetFromStorage("net/disco/.A")
    if err != nil {
        t.Error("Error returned from etcd", err)
        t.Fatal()
    }

    if len(nodes) != 1 {
        t.Error("Number of nodes should be 1: ", len(nodes))
        t.Fatal()
    }

    node := nodes[0]
    if node.Value != "1.1.1.1" {
        t.Error("Node value should be 1.1.1.1: ", node)
        t.Fail()
    }
}

func TestGetFromStorageNestedKeys(t *testing.T) {
    resolver.etcdPrefix = "TestGetFromStorageNestedKeys/"
    client.Set("TestGetFromStorageNestedKeys/net/disco/.A/0", "1.1.1.1", 0)
    client.Set("TestGetFromStorageNestedKeys/net/disco/.A/1", "1.1.1.2", 0)
    client.Set("TestGetFromStorageNestedKeys/net/disco/.A/2/0", "1.1.1.3", 0)

    nodes, err := resolver.GetFromStorage("net/disco/.A")
    if err != nil {
        t.Error("Error returned from etcd", err)
        t.Fatal()
    }

    if len(nodes) != 3 {
        t.Error("Number of nodes should be 3: ", len(nodes))
        t.Fatal()
    }

    var node *etcd.Node

    node = nodes[0]
    if node.Value != "1.1.1.1" {
        t.Error("Node value should be 1.1.1.1: ", node)
        t.Fail()
    }
    node = nodes[1]
    if node.Value != "1.1.1.2" {
        t.Error("Node value should be 1.1.1.2: ", node)
        t.Fail()
    }
    node = nodes[2]
    if node.Value != "1.1.1.3" {
        t.Error("Node value should be 1.1.1.3: ", node)
        t.Fail()
    }
}

func TestNameToKeyConverter(t *testing.T) {
    var key string

    key = nameToKey("foo.net.", "")
    if key != "/net/foo" {
        t.Error("Expected key /net/foo")
    }

    key = nameToKey("foo.net", "")
    if key != "/net/foo" {
        t.Error("Expected key /net/foo")
    }

    key = nameToKey("foo.net.", "/.A")
    if key != "/net/foo/.A" {
        t.Error("Expected key /net/foo/.A")
    }
}

/**
 * Test that the right authority is being returned for different types of DNS
 * queries.
 */

func TestAuthorityRoot(t *testing.T) {
    resolver.etcdPrefix = "TestAuthorityRoot/"
    client.Set("TestAuthorityRoot/net/disco/.SOA", "ns1.disco.net.\\tadmin.disco.net.\\t3600\\t600\\t86400\\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Expected zero answers")
        t.Fail()
    }

    if len(answer.Ns) != 1 {
        t.Error("Expected one authority record")
        t.Fail()
    }

    rr := answer.Ns[0].(*dns.SOA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fail()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fail()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fail()
    }
    // if rr.Serial != "admin.disco.net" {
    //     t.Error("Expected MBOX to be admin.disco.net: ", rr.Mbox)
    // }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fail()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fail()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fail()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fail()
    }
}

func TestAuthorityDomain(t *testing.T) {
    resolver.etcdPrefix = "TestAuthorityDomain/"
    client.Set("TestAuthorityDomain/net/disco/.SOA", "ns1.disco.net.\\tadmin.disco.net.\\t3600\\t600\\t86400\\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Expected zero answers")
        t.Fail()
    }

    if len(answer.Ns) != 1 {
        t.Error("Expected one authority record")
        t.Fail()
    }

    rr := answer.Ns[0].(*dns.SOA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fail()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fail()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fail()
    }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fail()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fail()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fail()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fail()
    }
}

/**
 * Test different that types of DNS queries return the correct answers
 **/

func TestAnswerQuestionA(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionA/"
    client.Set("TestAnswerQuestionA/net/disco/bar/.A", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionA/net/disco/.SOA", "ns1.disco.net.\\tadmin.disco.net.\\t3600\\t600\\t86400\\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 1 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fail()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fail()
    }

    rr := answer.Answer[0].(*dns.A)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fail()
    }

    // Verify the record itself is correct
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fail()
    }
}

func TestAnswerQuestionAAAA(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionAAAA/"
    client.Set("TestAnswerQuestionAAAA/net/disco/bar/.AAAA", "::1", 0)
    client.Set("TestAnswerQuestionAAAA/net/disco/.SOA", "ns1.disco.net.\\tadmin.disco.net.\\t3600\\t600\\t86400\\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeAAAA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 1 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fail()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fail()
    }

    rr := answer.Answer[0].(*dns.AAAA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeAAAA {
        t.Error("Expected record with type AAAA:", header.Rrtype)
        t.Fail()
    }

    // Verify the record itself is correct
    if rr.AAAA.String() != "::1" {
        t.Error("Expected AAAA record to be 1.2.3.4: ", rr.AAAA)
        t.Fail()
    }
}

func TestAnswerQuestionANY(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionANY/"
    client.Set("TestAnswerQuestionANY/net/disco/bar/.TXT", "google.com.", 0)
    client.Set("TestAnswerQuestionANY/net/disco/bar/.A/0", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionANY/net/disco/bar/.A/1", "2.3.4.5", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeANY)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 3 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fail()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fail()
    }
}

/**
 * Test converstion of names (i.e etcd nodes) to single records of different
 * types.
 **/

func TestLookupAnswerForA(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForA/"
    client.Set("TestLookupAnswerForA/net/disco/bar/.A", "1.2.3.4", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fail()
    }

    rr := records[0].(*dns.A)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fail()
    }
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fail()
    }
}

func TestLookupAnswerForAAAA(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForAAAA/"
    client.Set("TestLookupAnswerForAAAA/net/disco/bar/.AAAA", "::1", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeAAAA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fail()
    }

    rr := records[0].(*dns.AAAA)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeAAAA {
        t.Error("Expected record with type AAAA:", header.Rrtype)
        t.Fail()
    }
    if rr.AAAA.String() != "::1" {
        t.Error("Expected AAAA record to be ::1: ", rr.AAAA)
        t.Fail()
    }
}

func TestLookupAnswerForCNAME(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForCNAME/"
    client.Set("TestLookupAnswerForCNAME/net/disco/bar/.CNAME", "cname.google.com.", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeCNAME)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fail()
    }

    rr := records[0].(*dns.CNAME)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeCNAME {
        t.Error("Expected record with type CNAME:", header.Rrtype)
        t.Fail()
    }
    if rr.Target != "cname.google.com." {
        t.Error("Expected CNAME record to be cname.google.com.: ", rr.Target)
        t.Fail()
    }
}

func TestLookupAnswerForNS(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForNS/"
    client.Set("TestLookupAnswerForNS/net/disco/bar/.NS", "dns.google.com.", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeNS)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fail()
    }

    rr := records[0].(*dns.NS)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeNS {
        t.Error("Expected record with type NS:", header.Rrtype)
        t.Fail()
    }
    if rr.Ns != "dns.google.com." {
        t.Error("Expected NS record to be dns.google.com.: ", rr.Ns)
        t.Fail()
    }
}

func TestLookupAnswerForSOA(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForSOA/"
    client.Set("TestLookupAnswerForSOA/net/disco/.SOA", "ns1.disco.net.\\tadmin.disco.net.\\t3600\\t600\\t86400\\t10", 0)

    records, _ := resolver.LookupAnswersForType("disco.net.", dns.TypeSOA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fail()
    }

    rr := records[0].(*dns.SOA)
    header := rr.Header()

    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fail()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fail()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fail()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fail()
    }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fail()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fail()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fail()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fail()
    }
}
