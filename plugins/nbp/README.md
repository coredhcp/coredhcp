# Network Boot Program (NBP)

The `nbp` plugin adds boot information requested by DHCP clients.

## DHCPv4

```yaml
server4:
  plugins:
    - server_id: 192.0.2.1
    - nbp: https://ztp.example/ztp.json continue
    - file: /etc/coredhcp/hosts
```

The optional `continue` argument makes `nbp` add the requested boot options
and allow subsequent plugins to run, even if the client requested no boot
options. Place it before address-allocation plugins such as `file` or `range`.
Configure the other lease and network options required by your clients as usual.

Without `continue`, the existing one-URL form terminates the handler chain.
Adding boot options does not allocate an address. Continuation also does not
guarantee allocation: a later allocator must recognize the client or have an
available address. Later plugins may overwrite options added by `nbp`.

For HTTP, HTTPS, or FTP, the complete URL is returned in option 67. Other
schemes retain the existing TFTP behavior: the host is returned in option 66
and the path in option 67. The client's parameter request list is respected;
if it is absent, existing behavior includes all applicable boot options.

NBP does not filter clients by user class. Use an appropriate listener or network
scope when a URL is intended only for provisioning devices. Device classification
and selection of different boot URLs are separate concerns.

## DHCPv6

DHCPv6 keeps the existing single-URL syntax and terminal behavior; the
`continue` argument is supported only for DHCPv4.

Related: https://github.com/coredhcp/coredhcp/issues/289
