package main

import (
	"github.com/miekg/dns"
)

func LookupHandler(response dns.ResponseWriter, req *dns.Msg) {
	q := req.Question[0]
	r := &Resolver{}

    var answers []dns.RR

	if q.Qclass == dns.ClassINET {
		if q.Qtype == dns.TypeA {
			logger.Printf("Q: A record for %s", q.Name)
			answers = r.LookupA(q.Name)
		}
		// if q.Qtype == dns.TypeTXT {
		// 	logger.Printf("Q: TXT record for %s", q.Name)
		// 	answers = r.LookupA(q.Name)
		// }
		// if q.Qtype == dns.TypeCNAME {
		// 	logger.Printf("Q: CNAME record for %s", q.Name)
		// 	answers = r.LookupA(q.Name)
		// }
	}

	m := new(dns.Msg)
	m.SetReply(req)

	for _, a := range answers {
		header := a.Header()
		header.Name = q.Name
		header.Class = q.Qclass
		header.Rrtype = q.Qtype

		m.Answer = append(m.Answer, a)
	}

	response.WriteMsg(m)
}
