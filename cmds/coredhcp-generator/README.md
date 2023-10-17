## CoreDHCP Generator

`coredhcp-generator` is a tool used to build CoreDHCP with the plugins you want.

Why is it even needed? Go is a compiled language with no dynamic loading
support. In order to load a plugin, it has to be compiled in. We are happy to
provide a standard [main.go](/cmds/coredhcp/main.go), and at the same time we
don't want to include plugins that not everyone would use, otherwise the binary
size would grow without control.

You can use `coredhcp-generator` to generate a `main.go` that includes all the
plugins you wish. Just use it as follows:

```
$ ./coredhcp-generator --from core-plugins.txt
2019/11/21 23:32:04 Generating output file '/tmp/coredhcp547019106/coredhcp.go' with 7 plugin(s):
2019/11/21 23:32:04   1) github.com/coredhcp/coredhcp/plugins/file
2019/11/21 23:32:04   2) github.com/coredhcp/coredhcp/plugins/lease_time
2019/11/21 23:32:04   3) github.com/coredhcp/coredhcp/plugins/netmask
2019/11/21 23:32:04   4) github.com/coredhcp/coredhcp/plugins/range
2019/11/21 23:32:04   5) github.com/coredhcp/coredhcp/plugins/router
2019/11/21 23:32:04   6) github.com/coredhcp/coredhcp/plugins/server_id
2019/11/21 23:32:04   7) github.com/coredhcp/coredhcp/plugins/dns
2019/11/21 23:32:04 Generated file '/tmp/coredhcp547019106/coredhcp.go'. You can build it by running 'go build' in the output directory.
```

You can also specify the plugin list on the command line, or mix it with
`--from`:
```
$ ./coredhcp-generator --from core-plugins.txt \
    github.com/coredhcp/plugins/redis
```

Notice that it created a file called `coredhcp.go` in a temporary directory. You
can now `go build` that file and have your own custom CoreDHCP.

## Bugs

CoreDHCP uses Go versioned modules. The generated file does not do that yet. We
will add this feature soon.
