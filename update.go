package main

import (
    "crypto/md5"
    "encoding/hex"
    "github.com/coreos/go-etcd/etcd"
    "github.com/miekg/dns"
    "fmt"
    "strconv"
    "strings"
)

type DynamicUpdateManager struct {
    etcd        *etcd.Client
    etcdPrefix  string
    resolver    *Resolver
}

// Update will perform the necessary logic to update the DNS database with
// the changes described in the RFC-2136 formatted DNS message given.
// The return value will be the response message to send back to the client.
// It is assumed at this level the client has already authenticated and proven
// their right to update records in the given zone.
func (u *DynamicUpdateManager) Update(zone string, req *dns.Msg) (msg *dns.Msg) {

    rrsets := [][]dns.RR{req.Answer, req.Ns}
    msg = new(dns.Msg)
    msg.SetReply(req)

    // dns update re-aporopriates DNS message blocks:
    // msg.Question: Zone info for whole request
    // msg.Answer:   prerequisites
    // msg.Ns:       the actual update RRs

    // Verify the updates are within the zone we're modifying, since cross
    // zone updates are invalid.
    for _, rrs := range rrsets {
        for _, rr := range rrs {
            if dns.CompareDomainName(rr.Header().Name, zone) != dns.CountLabel(zone) {
                debugMsg("Domain " + rr.Header().Name + " is not in the " + zone + " zone")
                msg.SetRcode(req, dns.RcodeNotZone)
                return
            }
        }
    }

    // Ensure we recover from any panicking goroutine
    defer func() {
        if r := recover(); r != nil {
            debugMsg("[PANIC] " + fmt.Sprint(r))
            msg.SetRcode(req, dns.RcodeServerFailure)
        }
    }()

    // Attempt to acquire the dns-updates lock key.
    // TODO (orls): This means all updates from all running instances are
    // applied fully serially; this is less than ideal, the spec says they
    // should be serial only when conflicting with one another. But...this is
    // easier than building full transactions isolation mgmt :) For a low
    // frequency of updates, this should fine.
    lock := NewEtcdKeyLock(u.etcd, u.etcdPrefix + "._DISCODNS_UPDATE_LOCK")
    defer lock.Abandon()
    // block until locked or timed-out
    _, err := lock.WaitForAcquire(30)
    if err != nil {
        debugMsg("Failed to acquire or keep the update lock: ", err)
        msg.SetRcode(req, dns.RcodeServerFailure)
        return
    }

    // Validate the prerequisites of the update, returning immediately if they
    // are not satisfied.
    prereqValidation := validatePrerequisites(req.Answer, u.resolver)
    if prereqValidation != dns.RcodeSuccess {
        debugMsg("Validation of prerequisites failed")
        msg.SetRcode(req, prereqValidation)
        return
    }

    updateValidation := validateUpdates(req.Ns, req.Question[0])
    if updateValidation != dns.RcodeSuccess {
        debugMsg("Validation of update instructions failed")
        msg.SetRcode(req, updateValidation)
        return
    }

    // Perform the updates to the domain name system
    // This is not inside any kind of transaction, so a failure here *can*
    // result in a partially updated zone.
    // TODO(tarnfeld): Figure out a way of rolling back changes, perhaps make
    // use of the etcd indexes?
    msg.SetRcode(req, performUpdate(u.etcdPrefix, u.etcd, u.resolver, req.Question[0], req.Ns))

    return
}

// internal utility struct for making a map of RRsets a bit neater to construct
type matchKey struct {
    name string
    rrType uint16
}

// validatePrerequisites will perform all necessary validation checks against
// update prerequisites and return the relevant status is validation fails,
// otherwise NOERROR(0) will be returned.
// See RFC 2136, section 3.2
func validatePrerequisites(rr []dns.RR, resolver *Resolver) (rcode int) {
    rrSetsToMatch := make(map[matchKey][]dns.RR)
    for _, record := range rr {
        header := record.Header()
        if header.Ttl != 0 {
            return dns.RcodeFormatError
        }

        if header.Class == dns.ClassANY {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            } else if header.Rrtype == dns.TypeANY {
                // RFC Meaning: "Name is in use"
                exists, err := resolver.NameExists(header.Name)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if !exists {
                    debugMsg("Domain that should exist does not ", header.Name)
                    return dns.RcodeNameError
                }
            } else {
                // RFC Meaning: "RRset exists (value independent)"
                exists, err := resolver.RRSetExists(header.Name, header.Rrtype)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if !exists {
                    debugMsg("RRset that should exist does not ", header.Name, header.Rrtype)
                    return dns.RcodeNXRrset
                }
            }
        } else if header.Class == dns.ClassNONE {
            if header.Rdlength != 0 {
                return dns.RcodeFormatError
            } else if header.Rrtype == dns.TypeANY {
                // RFC Meaning: "Name is not in use"
                exists, err := resolver.NameExists(header.Name)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if exists {
                    debugMsg("Domain that should not exist does ", header.Name)
                    return dns.RcodeYXDomain
                }
            } else {
                // RFC meaning: "RRset does not exist"
                exists, err := resolver.RRSetExists(header.Name, header.Rrtype)
                if err != nil {
                    return dns.RcodeServerFailure
                }
                if exists {
                    debugMsg("RRset that should not exist does ", header.Name)
                    return dns.RcodeYXRrset
                }
            }
        } else if header.Class == dns.ClassINET {
            if header.Rrtype == dns.TypeANY {
                return dns.RcodeFormatError
            } else {
                // RFC Meaning: "RRset exists (value dependent)"
                mKey := matchKey{name: header.Name, rrType: header.Rrtype}
                rrSetsToMatch[mKey] = append(rrSetsToMatch[mKey], record)
            }
        } else {
            return dns.RcodeFormatError
        }
    }

    for matchKey, rrs := range rrSetsToMatch {
        matched, err := resolver.RRSetMatches(matchKey.name, matchKey.rrType, rrs)
        if err != nil {
            return dns.RcodeServerFailure
        }
        if !matched {
            return dns.RcodeNXRrset
        }
    }

    return dns.RcodeSuccess
}

// validateUpdates ensures that the given update instructions conform to the RFC
// and are processable, before we begin mutating state
// See RFC 2136, section 3.4.1
func validateUpdates(rrs []dns.RR, updateZone dns.Question) (rcode int) {

    // name-in-zone checks have already been performed.

    badTypes := map[uint16]bool{ dns.TypeIXFR : true,
        dns.TypeAXFR : true, dns.TypeMAILB : true, dns.TypeMAILA : true,
        dns.TypeANY : true}
    anyClsBadTypes := map[uint16]bool{ dns.TypeIXFR : true,
        dns.TypeAXFR : true, dns.TypeMAILB : true,dns.TypeMAILA : true}

    for _, rr := range rrs {
        header := rr.Header()
        if header.Class == updateZone.Qclass {
            if badTypes[header.Rrtype] {
                debugMsg("Bad type for class:", dns.ClassToString[header.Class],
                    header.Name, dns.TypeToString[header.Rrtype])
                return dns.RcodeFormatError
            }
        } else if header.Class == dns.ClassANY {
            if header.Ttl != 0 || header.Rdlength != 0 || anyClsBadTypes[header.Rrtype] {
                debugMsg("Bad ttl/length/type for class:", dns.ClassToString[header.Class],
                    header.Name, header.Ttl, header.Rdlength, dns.TypeToString[header.Rrtype])
                return dns.RcodeFormatError
            }
        } else if header.Class == dns.ClassNONE {
            if header.Ttl != 0 || badTypes[header.Rrtype] {
                debugMsg("Bad ttl/type for class:", dns.ClassToString[header.Class],
                    header.Name, header.Ttl, dns.TypeToString[header.Rrtype])
                return dns.RcodeFormatError
            }
        } else {
            return dns.RcodeFormatError
        }
        // separately from the RFC validation, fail for RR types we don't understand yet
        if _, ok := convertersFromRR[header.Rrtype]; ok != true {
            debugMsg("Record converter doesn't exist for " + dns.TypeToString[header.Rrtype])
            return dns.RcodeServerFailure
        }
    }
    return dns.RcodeSuccess
}

// performUpdate will commit the requested updates to the database
// It is assumed by this point all prerequisites have been validated and all
// domains are locked.
// See RFC 2136, section 3.4.2
func performUpdate(prefix string, etcdClient *etcd.Client, resolver *Resolver, updateZone dns.Question, records []dns.RR) (rcode int) {

    for _, rr := range records {
        header := rr.Header()
        if _, ok := convertersFromRR[header.Rrtype]; ok != true {
            panic("Record converter doesn't exist for " + dns.TypeToString[header.Rrtype])
        }

        nameDir := nameToKey(header.Name, "")

        // Gather up all deletes
        if header.Class == dns.ClassANY {
            var typesToDelete []uint16
            if header.Rrtype == dns.TypeANY {
                // RFC Meaning: Delete all RRsets from a name

                // Look up all RRs for supported types. Do this 'manually' because we
                // don't want standard resolver behaviour about CNAMEs
                // TODO: this means it's not parallelized like it normally is
                for rrType, _ := range convertersToRR {
                    if header.Name == updateZone.Name && (rrType == dns.TypeNS || rrType == dns.TypeSOA) {
                        // RFC explicitly forbids deleting NS/SOA for the zone in this way
                        continue
                    }
                    typesToDelete = append(typesToDelete, rrType)
                }
            } else {
                // RFC Meaning: Delete an RRset (all RRs of type)
                typesToDelete = append(typesToDelete, header.Rrtype)
            }
            for _, rrType := range typesToDelete {
                records, err := resolver.GetFromStorage(nameDir + "/." + dns.TypeToString[rrType])
                if err != nil && !missingKeyErr(err) {
                    debugMsg(err)
                    panic("Failed to fetch existing records for " + nameDir)
                }
                for _, toDelete := range records {
                    deleteKeyAndTtl(etcdClient, toDelete.node.Key)
                }
            }
        } else {

            updateNode, err := convertRRToNode(rr, *header)
            if err != nil {
                panic("Got error when converting node")
            } else if updateNode == nil {
                panic("Got NIL after successfully converting node")
            }

            existingRecords, err := resolver.GetFromStorage(updateNode.Key)
            if err != nil && !missingKeyErr(err) {
                debugMsg(err)
                panic("Failed to fetch existing RRs for key " + updateNode.Key)
            }

            if header.Class == dns.ClassNONE {
                // RFC Meaning: Delete an RR from an RRset
                for _, existing := range existingRecords {
                    if existing.node.Value == updateNode.Value {
                        deleteKeyAndTtl(etcdClient, existing.node.Key)
                    }
                }
            } else {
                // RFC Meaning: Add to an RRset

                // Ignore certain inserts in presence of CNAMEs, as per RFC.
                // Further explanation from http://docs.freebsd.org/doc/8.0-RELEASE/usr/share/doc/bind9/arm/man.nsupdate.html :
                // "...cannot conflict with the long-standing rule in RFC1034 that a name must not exist as any other
                // record type if it exists as a CNAME. (The rule has been updated for DNSSEC in RFC2535 to allow
                // CNAMEs to have RRSIG, DNSKEY and NSEC records.)"
                // TODO(orls): add these special cases to the special case for CNAMEs. Yayyyyy standards

                hasCNAME := false
                hasNonCNAME := false
                neighbouringTypes, err := etcdClient.Get(prefix + nameDir, false, false)
                if err == nil {
                    for _, n := range neighbouringTypes.Node.Nodes {
                        splits := strings.Split(n.Key, nameDir + "/.")
                        if len(splits) != 2  || strings.HasSuffix(splits[1], ".ttl") {
                            continue
                        }
                        if splits[1] == "CNAME" {
                            hasCNAME = true
                        } else {
                            hasNonCNAME = true
                        }
                    }
                }

                if header.Rrtype == dns.TypeCNAME && hasNonCNAME {
                    debugMsg("Ignoring insert for CNAME due to existing non-CNAME record(s) for " + header.Name)
                    continue
                } else if header.Rrtype != dns.TypeCNAME && hasCNAME {
                    debugMsg("Ignoring insert for " + dns.TypeToString[header.Rrtype] + " due to existing CNAME record(s) for " + header.Name)
                    continue
                }

                foundExisting := false
                var ttlKeys []string

                // Check for existing matching records, in which case just update TTL.
                // Otherwise there's risk of duplicates
                for _, existing := range existingRecords {
                    if existing.node.Value == updateNode.Value {
                        debugMsg("update req matched existing rr with key " + existing.node.Key)
                        foundExisting = true
                        ttlKeys = append(ttlKeys, existing.node.Key + ".ttl")
                    }
                }

                if !foundExisting {
                    // Then we need to add, which means we need a directory.
                    // Convert any old-style single-keys to directories
                    if len(existingRecords) == 1 && existingRecords[0].node.Key == "/" + prefix + updateNode.Key {
                        originalNode := existingRecords[0].node
                        logger.Printf("[WARNING] ------")
                        logger.Printf("[WARNING] Converting existing value to a directory!")
                        logger.Printf("[WARNING] Existing record is old-style single key: " + originalNode.Key)
                        logger.Printf("[WARNING] ------")

                        convertedKey := originalNode.Key + "/" + recordSubkey(originalNode.Value)
                        _, convertErr := etcdClient.SetDir(originalNode.Key, 0)
                        if convertErr != nil {
                            debugMsg(convertErr)
                            // panic("Failed to insert record into etcd")
                        }
                        _, convertErr = etcdClient.Set(convertedKey, originalNode.Value, 0)
                        if convertErr != nil {
                            debugMsg(convertErr)
                            // panic("Failed to insert record into etcd")
                        }
                        if existingRecords[0].ttl != 0 {
                            convertTTL := strconv.FormatInt(int64(existingRecords[0].ttl), 10)
                            _, convertErr = etcdClient.Set(convertedKey + ".ttl", convertTTL, 0)
                            if convertErr != nil {
                                debugMsg(convertErr)
                                // panic("Failed to insert record into etcd")
                            }
                        }
                    }

                    newKey := prefix + updateNode.Key + "/" + recordSubkey(updateNode.Value)
                    ttlKeys = append(ttlKeys, newKey + ".ttl")

                    debugMsg("Inserting new record to " + newKey)
                    _, err := etcdClient.Set(newKey, updateNode.Value, 0)
                    if err != nil {
                        debugMsg(err)
                        // panic("Failed to insert record into etcd")
                    }
                }

                // Insert the TTL record if one has been requested
                if header.Ttl > 0 {
                    ttl := strconv.FormatInt(int64(header.Ttl), 10)
                    for _, ttlKey := range ttlKeys {
                        debugMsg("Inserting/updating TTL key " + ttlKey)
                        _, err = etcdClient.Set(ttlKey, ttl, 0)
                        if err != nil {
                            debugMsg(err)
                            // panic("Failed to insert ttl into etcd")
                        }
                    }
                }
            }
        }
    }

    return dns.RcodeSuccess
}

// Internal boileplate-reducer to delete a specific key (non-recursive) and
// handle it's TTL key too
func deleteKeyAndTtl(etcdClient *etcd.Client, delKey string) {
    delTTLKey := delKey + ".ttl"
    debugMsg("Deleting RR with key " + delKey)
    _, err := etcdClient.Delete(delKey, true)
    if err != nil && !missingKeyErr(err) {
        debugMsg(err)
        panic("Failed to delete RRs with key " + delKey)
    }
    _, err = etcdClient.Delete(delTTLKey, true)
    if err != nil && !missingKeyErr(err) {
        debugMsg(err)
        panic("Failed to delete RR TTL key " + delTTLKey)
    }
}

// recordSubkey yields the sub-key string to be used for a new RR in a
// directory, based on it's data. The MD5 of the node value is used, making
// duplicates impossible.
func recordSubkey(value string) (subkey string) {
    hasher := md5.New()
    hasher.Write([]byte(value))
    return hex.EncodeToString(hasher.Sum(nil))
}

// internal helper to determine if an error from etcd operations is a 100-code
// error, i.e. that the key is missing.
func missingKeyErr(err error) (ok bool) {
    etcdErr, cast := err.(*etcd.EtcdError)
    if cast && etcdErr.ErrorCode == 100 {
        return true
    }
    return false
}
