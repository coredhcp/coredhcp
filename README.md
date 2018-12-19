# coredhcp

[![Build Status](https://travis-ci.org/coredhcp/coredhcp.svg?branch=master)](https://travis-ci.org/coredhcp/coredhcp)
[![codecov](https://codecov.io/gh/coredhcp/coredhcp/branch/master/graph/badge.svg)](https://codecov.io/gh/coredhcp/coredhcp)
[![Go Report Card](https://goreportcard.com/badge/github.com/coredhcp/coredhcp)](https://goreportcard.com/report/github.com/coredhcp/coredhcp)

Fast, multithreaded, modular and extensible DHCP server written in Go

This is still a work-in-progress

## Example configuration

In CoreDHCP almost everything is implemented as a plugin. The order of plugins in the configuration matters: every request is evaluated calling each plugin in order, until one breaks the evaluation and responds to, or drops, the request.

The following configuration runs a DHCPv6-only server, listening on all the interfaces, using a custom DUID-LL as server ID, and reading the leases from a text file.

```
server6:
    listen: '[::]:547'
    plugins:
        - server_id: LL 00:de:ad:be:ef:00
        - file: "leases.txt"
        # - dns: 8.8.8.8 8.8.4.4 2001:4860:4860::8888 2001:4860:4860::8844

#server4:
#    listen: '127.0.0.1:67'
```

See also [config.yml.example](config.yml.example).
