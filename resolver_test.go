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
    if node.node.Value != "1.1.1.1" {
        t.Error("Node value should be 1.1.1.1: ", node)
        t.Fatal()
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

    var node *EtcdRecord

    node = nodes[0]
    if node.node.Value != "1.1.1.1" {
        t.Error("Node value should be 1.1.1.1: ", node)
        t.Fatal()
    }
    node = nodes[1]
    if node.node.Value != "1.1.1.2" {
        t.Error("Node value should be 1.1.1.2: ", node)
        t.Fatal()
    }
    node = nodes[2]
    if node.node.Value != "1.1.1.3" {
        t.Error("Node value should be 1.1.1.3: ", node)
        t.Fatal()
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
    client.Set("TestAuthorityRoot/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Expected zero answers")
        t.Fatal()
    }

    if len(answer.Ns) != 1 {
        t.Error("Expected one authority record")
        t.Fatal()
    }

    rr := answer.Ns[0].(*dns.SOA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fatal()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fatal()
    }
    // if rr.Serial != "admin.disco.net" {
    //     t.Error("Expected MBOX to be admin.disco.net: ", rr.Mbox)
    // }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fatal()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fatal()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fatal()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fatal()
    }
}

func TestAuthorityDomain(t *testing.T) {
    resolver.etcdPrefix = "TestAuthorityDomain/"
    client.Set("TestAuthorityDomain/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Expected zero answers")
        t.Fatal()
    }

    if len(answer.Ns) != 1 {
        t.Error("Expected one authority record")
        t.Fatal()
    }

    rr := answer.Ns[0].(*dns.SOA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fatal()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fatal()
    }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fatal()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fatal()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fatal()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fatal()
    }
}

func TestAuthoritySubdomain(t *testing.T) {
    resolver.etcdPrefix = "TestAuthoritySubdomain/"
    client.Set("TestAuthoritySubdomain/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)
    client.Set("TestAuthoritySubdomain/net/disco/bar/.SOA", "ns1.bar.disco.net.\tbar.disco.net.\t3600\t600\t86400\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("foo.bar.disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Expected zero answers")
        t.Fatal()
    }

    if len(answer.Ns) != 1 {
        t.Error("Expected one authority record")
        t.Fatal()
    }

    rr := answer.Ns[0].(*dns.SOA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.bar.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fatal()
    }
    if rr.Mbox != "bar.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fatal()
    }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fatal()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fatal()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fatal()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fatal()
    }
}

/**
 * Test different that types of DNS queries return the correct answers
 **/

func TestAnswerQuestionA(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionA/"
    client.Set("TestAnswerQuestionA/net/disco/bar/.A", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionA/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 1 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fatal()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fatal()
    }

    rr := answer.Answer[0].(*dns.A)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fatal()
    }
}

func TestAnswerQuestionAAAA(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionAAAA/"
    client.Set("TestAnswerQuestionAAAA/net/disco/bar/.AAAA", "::1", 0)
    client.Set("TestAnswerQuestionAAAA/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeAAAA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 1 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fatal()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fatal()
    }

    rr := answer.Answer[0].(*dns.AAAA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeAAAA {
        t.Error("Expected record with type AAAA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.AAAA.String() != "::1" {
        t.Error("Expected AAAA record to be ::1: ", rr.AAAA)
        t.Fatal()
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
        t.Fatal()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fatal()
    }
}

func TestAnswerQuestionWildcardAAAANoMatch(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionWildcardANoMatch/"
    client.Set("TestAnswerQuestionWildcardANoMatch/net/disco/bar/*/.AAAA", "::1", 0)

    query := new(dns.Msg)
    query.SetQuestion("bar.disco.net.", dns.TypeAAAA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) > 0 {
        t.Error("Didn't expect any answers, got ", len(answer.Answer))
        t.Fatal()
    }
}

func TestAnswerQuestionWildcardAAAA(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionWildcardA/"
    client.Set("TestAnswerQuestionWildcardA/net/disco/bar/*/.AAAA", "::1", 0)

    query := new(dns.Msg)
    query.SetQuestion("baz.bar.disco.net.", dns.TypeAAAA)

    answer := resolver.Lookup(query)

    if len(answer.Answer) != 1 {
        t.Error("Expected one answer, got ", len(answer.Answer))
        t.Fatal()
    }

    if len(answer.Ns) > 0 {
        t.Error("Didn't expect any authority records")
        t.Fatal()
    }

    rr := answer.Answer[0].(*dns.AAAA)
    header := rr.Header()

    // Verify the header is correct
    if header.Name != "baz.bar.disco.net." {
        t.Error("Expected record with name baz.bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeAAAA {
        t.Error("Expected record with type AAAA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.AAAA.String() != "::1" {
        t.Error("Expected AAAA record to be ::1: ", rr.AAAA)
        t.Fatal()
    }
}

func TestAnswerQuestionTTL(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionTTL/"
    client.Set("TestAnswerQuestionTTL/net/disco/bar/.A", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionTTL/net/disco/bar/.A.ttl", "300", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.A)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fatal()
    }
    if header.Ttl != 300 {
        t.Error("Expected TTL of 300 seconds:", header.Ttl)
        t.Fatal()
    }
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fatal()
    }
}

func TestAnswerQuestionTTLMultipleRecords(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionTTLMultipleRecords/"
    client.Set("TestAnswerQuestionTTLMultipleRecords/net/disco/bar/.A/0", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionTTLMultipleRecords/net/disco/bar/.A/0.ttl", "300", 0)
    client.Set("TestAnswerQuestionTTLMultipleRecords/net/disco/bar/.A/1", "8.8.8.8", 0)
    client.Set("TestAnswerQuestionTTLMultipleRecords/net/disco/bar/.A/1.ttl", "600", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeA)

    if len(records) != 2 {
        t.Error("Expected two answers, got ", len(records))
        t.Fatal()
    }

    rrOne := records[0].(*dns.A)
    headerOne := rrOne.Header()

    if headerOne.Ttl != 300 {
        t.Error("Expected TTL of 300 seconds:", headerOne.Ttl)
        t.Fatal()
    }
    if rrOne.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rrOne.A)
        t.Fatal()
    }

    rrTwo := records[1].(*dns.A)
    headerTwo := rrTwo.Header()

    if headerTwo.Ttl != 600 {
        t.Error("Expected TTL of 300 seconds:", headerTwo.Ttl)
        t.Fatal()
    }
    if rrTwo.A.String() != "8.8.8.8" {
        t.Error("Expected A record to be 8.8.8.8: ", rrTwo.A)
        t.Fatal()
    }
}

func TestAnswerQuestionTTLInvalidFormat(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionTTL/"
    client.Set("TestAnswerQuestionTTL/net/disco/bar/.A", "1.2.3.4", 0)
    client.Set("TestAnswerQuestionTTL/net/disco/bar/.A.ttl", "haha", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.A)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fatal()
    }
    if header.Ttl != 0 {
        t.Error("Expected TTL of 0 seconds:", header.Ttl)
        t.Fatal()
    }
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fatal()
    }
}

func TestAnswerQuestionTTLDanglingNode(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionTTLDanglingNode/"
    client.Set("TestAnswerQuestionTTLDanglingNode/net/disco/bar/.TXT.ttl", "600", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeTXT)

    if len(records) != 0 {
        t.Error("Expected no answer, got ", len(records))
        t.Fatal()
    }
}

func TestAnswerQuestionTTLDanglingDirNode(t *testing.T) {
    resolver.etcdPrefix = "TestAnswerQuestionTTLDanglingDirNode/"
    client.Set("TestAnswerQuestionTTLDanglingDirNode/net/disco/bar/.TXT/0.ttl", "600", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeTXT)

    if len(records) != 0 {
        t.Error("Expected no answer, got ", len(records))
        t.Fatal()
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
        t.Fatal()
    }

    rr := records[0].(*dns.A)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeA {
        t.Error("Expected record with type A:", header.Rrtype)
        t.Fatal()
    }
    if rr.A.String() != "1.2.3.4" {
        t.Error("Expected A record to be 1.2.3.4: ", rr.A)
        t.Fatal()
    }
}

func TestLookupAnswerForAAAA(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForAAAA/"
    client.Set("TestLookupAnswerForAAAA/net/disco/bar/.AAAA", "::1", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeAAAA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.AAAA)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeAAAA {
        t.Error("Expected record with type AAAA:", header.Rrtype)
        t.Fatal()
    }
    if rr.AAAA.String() != "::1" {
        t.Error("Expected AAAA record to be ::1: ", rr.AAAA)
        t.Fatal()
    }
}

func TestLookupAnswerForCNAME(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForCNAME/"
    client.Set("TestLookupAnswerForCNAME/net/disco/bar/.CNAME", "cname.google.com.", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeCNAME)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.CNAME)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeCNAME {
        t.Error("Expected record with type CNAME:", header.Rrtype)
        t.Fatal()
    }
    if rr.Target != "cname.google.com." {
        t.Error("Expected CNAME record to be cname.google.com.: ", rr.Target)
        t.Fatal()
    }
}

func TestLookupAnswerForNS(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForNS/"
    client.Set("TestLookupAnswerForNS/net/disco/bar/.NS", "dns.google.com.", 0)

    records, _ := resolver.LookupAnswersForType("bar.disco.net.", dns.TypeNS)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.NS)
    header := rr.Header()

    if header.Name != "bar.disco.net." {
        t.Error("Expected record with name bar.disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeNS {
        t.Error("Expected record with type NS:", header.Rrtype)
        t.Fatal()
    }
    if rr.Ns != "dns.google.com." {
        t.Error("Expected NS record to be dns.google.com.: ", rr.Ns)
        t.Fatal()
    }
}

func TestLookupAnswerForSOA(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForSOA/"
    client.Set("TestLookupAnswerForSOA/net/disco/.SOA", "ns1.disco.net.\tadmin.disco.net.\t3600\t600\t86400\t10", 0)

    records, _ := resolver.LookupAnswersForType("disco.net.", dns.TypeSOA)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.SOA)
    header := rr.Header()

    if header.Name != "disco.net." {
        t.Error("Expected record with name disco.net.: ", header.Name)
        t.Fatal()
    }
    if header.Rrtype != dns.TypeSOA {
        t.Error("Expected record with type SOA:", header.Rrtype)
        t.Fatal()
    }

    // Verify the record itself is correct
    if rr.Ns != "ns1.disco.net." {
        t.Error("Expected NS to be ns1.disco.net.: ", rr.Ns)
        t.Fatal()
    }
    if rr.Mbox != "admin.disco.net." {
        t.Error("Expected MBOX to be admin.disco.net.: ", rr.Mbox)
        t.Fatal()
    }
    if rr.Refresh != 3600 {
        t.Error("Expected REFRESH to be 3600: ", rr.Refresh)
        t.Fatal()
    }
    if rr.Retry != 600 {
        t.Error("Expected RETRY to be 600: ", rr.Retry)
        t.Fatal()
    }
    if rr.Expire != 86400 {
        t.Error("Expected EXPIRE to be 86400: ", rr.Expire)
        t.Fatal()
    }
    if rr.Minttl != 10 {
        t.Error("Expected MINTTL to be 10: ", rr.Minttl)
        t.Fatal()
    }
}

func TestLookupAnswerForPTR(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForPTR/"

    client.Set("TestLookupAnswerForPTR/net/disco/alias/.PTR/target1", "target1.disco.net.", 0)
    client.Set("TestLookupAnswerForPTR/net/disco/alias/.PTR/target2", "target2.disco.net.", 0)

    records, _ := resolver.LookupAnswersForType("alias.disco.net.", dns.TypePTR)

    if len(records) != 2 {
        t.Error("Expected two answers, got ", len(records))
        t.Fatal()
    }

    seen_1 := false
    seen_2 := false

    // We can't (and shouldn't try to) guarantee order, so check for all
    // expected records the long way
    for _, record := range records {
        rr := record.(*dns.PTR)
        header := rr.Header()

        if header.Rrtype != dns.TypePTR {
            t.Error("Expected record with type PTR:", header.Rrtype)
            t.Fatal()
        }

        t.Log(rr)

        if rr.Ptr == "target1.disco.net." {
            seen_1 = true
        }

        if rr.Ptr == "target2.disco.net." {
            seen_2 = true
        }
    }

    if seen_1 == false || seen_2 == false {
        t.Error("Didn't get back all expected PTR responses")
        t.Fatal()
    }
}

func TestLookupAnswerForPTRInvalidDomain(t *testing.T) {
    resolver.etcdPrefix = "TestLookupAnswerForPTRInvalidDomain/"

    client.Set("TestLookupAnswerForPTRInvalidDomain/net/disco/bad-alias/.PTR", "...", 0)

    records, err := resolver.LookupAnswersForType("bad-alias.disco.net.", dns.TypePTR)

    if len(records) > 0 {
        t.Error("Expected no answers, got ", len(records))
        t.Fatal()
    }

    if err == nil {
        t.Error("Expected error, didn't get one")
        t.Fatal()
    }
}

func TestLookupAnswerForSRV(t *testing.T) {

    resolver.etcdPrefix = "TestLookupAnswerForSRV/"
    client.Set("TestLookupAnswerForSRV/net/disco/_tcp/_http/.SRV",
        "100\t100\t80\tsome-webserver.disco.net",
        0)

    records, _ := resolver.LookupAnswersForType("_http._tcp.disco.net.", dns.TypeSRV)

    if len(records) != 1 {
        t.Error("Expected one answer, got ", len(records))
        t.Fatal()
    }

    rr := records[0].(*dns.SRV)

    if rr.Priority != 100 {
        t.Error("Unexpected 'priority' value for SRV record:", rr.Priority)
    }

    if rr.Weight != 100 {
        t.Error("Unexpected 'weight' value for SRV record:", rr.Weight)
    }

    if rr.Port != 80 {
        t.Error("Unexpected 'port' value for SRV record:", rr.Port)
    }

    if rr.Target != "some-webserver.disco.net." {
        t.Error("Unexpected 'target' value for SRV record:", rr.Target)
    }
}

func TestLookupAnswerForSRVInvalidValues(t *testing.T) {

    resolver.etcdPrefix = "TestLookupAnswerForSRVInvalidValues/"

    var bad_vals_map = map[string]string {
        "wrong-delimiter":      "10 10 80 foo.disco.net",
        "not-enough-fields":    "0\t0",
        "neg-int-priority":     "-10\t10\t80\tfoo.disco.net",
        "neg-int-weight":       "10\t-10\t80\tfoo.disco.net",
        "neg-int-port":         "10\t10\t-80\tfoo.disco.net",
        "large-int-priority":   "65536\t10\t80\tfoo.disco.net",
        "large-int-weight":     "10\t65536\t80\tfoo.disco.net",
        "large-int-port":       "10\t10\t65536\tfoo.disco.net"}

    for name, value := range bad_vals_map {

        client.Set("TestLookupAnswerForSRVInvalidValues/net/disco/" + name + "/.SRV", value, 0)
        records, err := resolver.LookupAnswersForType(name + ".disco.net.", dns.TypeSRV)

        if len(records) > 0 {
            t.Error("Expected no answers, got ", len(records))
            t.Fatal()
        }

        if err == nil {
            t.Error("Expected error, didn't get one")
            t.Fatal()
        }
    }
}
