package main

import (
    "github.com/miekg/dns"
    "strings"
)

type QueryFilter struct {
    domain          string
    qTypes          []string
}

type QueryFilterer struct {
    acceptFilters   []QueryFilter
    rejectFilters   []QueryFilter
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

// parseFilters will convert a string into a Query Filter structure. The accepted
// format for input is [domain]:[type,type,...]. For example...
// 
// - "domain:A,AAAA" # Match all A and AAAA queries within `domain`
// - ":TXT" # Matches only TXT queries for any domain
// - "domain:" # Matches any query within `domain`
func parseFilters(filters []string) []QueryFilter {
    parsedFilters := make([]QueryFilter, 0)
    for _, filter := range filters {
        components := strings.Split(filter, ":")
        if len(components) != 2 {
            logger.Printf("Expected only one colon ([domain]:[type,type...])")
            continue
        }

        domain := dns.Fqdn(components[0])
        types := strings.Split(components[1], ",")

        if len(types) == 1 && len(types[0]) == 0 {
            types = make([]string, 0)
        }

        debugMsg("Adding filter with domain '" + domain + "' and types '" + strings.Join(types, ",") + "'")
        parsedFilters = append(parsedFilters, QueryFilter{domain, types})
    }

    return parsedFilters
}
