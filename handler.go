package main

import (
	"github.com/miekg/dns"
)

func LookupHandler(response dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	r := &Resolver{}

    m := new(dns.Msg)
	m.SetReply(req)

	if q.Qclass == dns.ClassINET {
		if q.Qtype == dns.TypeA {
			logger.Printf("Q: A record for %s", q.Name)

			for _, a := range r.LookupA(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.A, header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}

		if q.Qtype == dns.TypeTXT {
			logger.Printf("Q: TXT record for %s", q.Name)

			for _, a := range r.LookupTXT(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.Txt[0], header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}

		if q.Qtype == dns.TypeCNAME {
			logger.Printf("Q: CNAME record for %s", q.Name)

			for _, a := range r.LookupCNAME(q.Name, q.Qclass, q.Qtype) {
				header := a.Header()
				logger.Printf("A: %s (TTL %d)", a.Target, header.Ttl)
				m.Answer = append(m.Answer, a)
			}
		}
	}

	response.WriteMsg(m)
}
