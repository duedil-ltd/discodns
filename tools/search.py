#!/usr/bin/env python
"""
Command line tool for searching the discodns domain space for references to
a given domain. For example, if you need to find all INCOMING CNAME records to
the domain foo.bar.net you could use this tool.
"""

import argparse
import json
import requests
import sys


def recurse_nodes(nodes):
    """Recursively yield all etcd nodes."""

    for node in nodes:
        yield node
        for sub_node in recurse_nodes(node.get("nodes", [])):
            yield sub_node


def list_recursive(base, key):
    url = "%s/%s?recursive=true" % (base, key)
    response = requests.get(url)
    if response.status_code >= 200 and response.status_code < 300:
        content = json.loads(response.text)
        root_node = content["node"]
        for node in recurse_nodes(root_node.get("nodes", [])):
            yield node


def format_node(node):
    return "%s => %s" % (node["key"], node.get("value"))


def main(args):
    parser = argparse.ArgumentParser(prog="search.py", description=__doc__)
    parser.add_argument("--etcd", default="127.0.0.1:4001",
                        help="Address for etcd server")
    parser.add_argument("query", help="Search query")

    args = parser.parse_args()
    print >> sys.stderr, "[INFO] Searching for domains at %s" % args.etcd

    etcd_keys_base = "http://%s/v2/keys" % args.etcd
    print >> sys.stderr, "[DEBUG] Using %s as the base for keys" % etcd_keys_base
    print "-" * 40

    reverse_domain_segments = args.query.split(".")[::-1]

    # Check for any entries for the domain itself
    for node in list_recursive(etcd_keys_base, "/".join(reverse_domain_segments)):
        print format_node(node)

    # Search for any domains which contain the query string
    for node in list_recursive(etcd_keys_base, "/"):
        if args.query in node.get("value", ""):
            print format_node(node)


if __name__ == "__main__":
    main(sys.argv[1:])
