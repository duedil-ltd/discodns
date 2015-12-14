package main

import (
	"github.com/miekg/dns"
	"strings"
)

type QueryFilter struct {
	domain string
	qTypes []string
}

type QueryFilterer struct {
	acceptFilters []QueryFilter
	rejectFilters []QueryFilter
}

// Matches returns true if the given DNS query matches the filter
func (f *QueryFilter) Matches(req *dns.Msg) bool {
	queryDomain := req.Question[0].Name
	queryQType := dns.TypeToString[req.Question[0].Qtype]
	if len(queryDomain) > 0 && !strings.HasSuffix(queryDomain, f.domain) {
		debugMsg("Domain match failed (" + queryDomain + ", " + f.domain + ")")
		return false
	}

	matches := false
	if len(f.qTypes) > 0 {
		for _, qType := range f.qTypes {
			if qType == queryQType {
				matches = true
			}
		}
	} else {
		matches = true
	}

	return matches
}

// ShouldAcceptQuery returns true if the given DNS query matches the given
// accept/reject filters, and should be accepted.
func (f *QueryFilterer) ShouldAcceptQuery(req *dns.Msg) bool {
	accepted := true

	for _, filter := range f.rejectFilters {
		filterDescription := "Filter " + filter.domain + ":" + strings.Join(filter.qTypes, ",")
		if filter.Matches(req) {
			debugMsg(filterDescription + " rejected")
			accepted = false
			break
		}
		debugMsg(filterDescription + " not rejected")
	}

	if accepted && len(f.acceptFilters) > 0 {
		accepted = false
		for _, filter := range f.acceptFilters {
			filterDescription := "Filter " + filter.domain + ":" + strings.Join(filter.qTypes, ",")
			if filter.Matches(req) {
				debugMsg(filterDescription + " accepted")
				accepted = true
				break
			}
			debugMsg(filterDescription + " not accepted")
		}
	}

	return accepted
}
