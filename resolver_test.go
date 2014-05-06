package main

import (
    "github.com/coreos/go-etcd/etcd"
    "testing"
)

var (
    client = etcd.NewClient([]string{"127.0.0.1:4001"})
    resolver = &Resolver{etcd: client}
)

func TestEtcd(t *testing.T) {
    if !client.SyncCluster() {
        t.Error("Failed to sync etcd cluster")
        t.Fatal()
    }
}

func TestGetFromStorageSingleKey(t *testing.T) {
    resolver.etcdPrefix = "foo/"
    client.Set("foo/net/disco/.A", "1.1.1.1", 0)

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

}

func TestAuthorityRoot(t *testing.T) {

}

func TestAuthorityDomain(t *testing.T) {

}

func TestLookup(t *testing.T) {
    
}

func TestAnswerQuestionANY(t *testing.T) {
    
}

func TestAnswerQuestionA(t *testing.T) {

}

func TestLookupAnswersForType(t *testing.T) {

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

func TestConvertersA(t *testing.T) {

}

func TestConvertersAAAA(t *testing.T) {
    
}

func TestConvertersCNAME(t *testing.T) {
    
}

func TestConvertersNS(t *testing.T) {
    
}

func TestConvertersSOA(t *testing.T) {
    
}
