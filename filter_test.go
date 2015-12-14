package main

import (
	"github.com/miekg/dns"
	"testing"
)

func TestFilters(t *testing.T) {
	// Enable debug logging
	log_debug = true
}

func TestNoFilters(t *testing.T) {
	filterer := QueryFilterer{}
	msg := generateDNSMessage("discodns.net", dns.TypeA)

	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}
}

func TestSimpleAccept(t *testing.T) {
	filterer := QueryFilterer{acceptFilters: parseFilters([]string{"net:A"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestSimpleReject(t *testing.T) {
	filterer := QueryFilterer{rejectFilters: parseFilters([]string{"net:A"})}

	msg := generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestSimpleAcceptFullDomain(t *testing.T) {
	filterer := QueryFilterer{acceptFilters: parseFilters([]string{"net:"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeANY)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeANY)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestSimpleRejectFullDomain(t *testing.T) {
	filterer := QueryFilterer{rejectFilters: parseFilters([]string{"net:"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeANY)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeANY)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}
}

func TestSimpleAcceptSpecificTypes(t *testing.T) {
	filterer := QueryFilterer{acceptFilters: parseFilters([]string{":A"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestSimpleAcceptMultipleTypes(t *testing.T) {
	filterer := QueryFilterer{acceptFilters: parseFilters([]string{":A,PTR"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypePTR)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}
}

func TestSimpleRejectSpecificTypes(t *testing.T) {
	filterer := QueryFilterer{rejectFilters: parseFilters([]string{":A"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}
}

func TestSimpleRejectMultipleTypes(t *testing.T) {
	filterer := QueryFilterer{rejectFilters: parseFilters([]string{":A,PTR"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypePTR)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestMultipleAccept(t *testing.T) {
	filterer := QueryFilterer{acceptFilters: parseFilters([]string{"net:A", "com:AAAA"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}
}

func TestMultipleReject(t *testing.T) {
	filterer := QueryFilterer{rejectFilters: parseFilters([]string{"net:A", "com:AAAA"})}

	msg := generateDNSMessage("discodns.net", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.net", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeAAAA)
	if filterer.ShouldAcceptQuery(msg) != false {
		t.Error("Expected the query to be rejected")
		t.Fatal()
	}

	msg = generateDNSMessage("discodns.com", dns.TypeA)
	if filterer.ShouldAcceptQuery(msg) != true {
		t.Error("Expected the query to be accepted")
		t.Fatal()
	}
}

// generateDNSMessage returns a simple DNS query with a single question,
// comprised of the domain and rrType given.
func generateDNSMessage(domain string, rrType uint16) *dns.Msg {
	domain = dns.Fqdn(domain)
	msg := dns.Msg{Question: []dns.Question{dns.Question{Name: domain, Qtype: rrType}}}
	return &msg
}
