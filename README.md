# Smykowski: Transparent http(s) whitelist proxy

A transparent http/https proxy with a whitelist to
limit traffic for all clients sending traffic through
it.

### Features

* Transparent
* HTTP(S) proxying
* Regex based whitelist
* Caches regex hits for faster subsequent requests
* Signal based whitelist reload (zero downtime update)

## How does it work?

#### SSL

If the client supports `SNI`, the proxy acts like a TLS
vhost to utilize SNI to get the `Host` from the client.
Then, it annotates the request with a `CONNECT` and hands
it off to `goproxy` (after checking the Host to make
sure it fits in the whitelist).

If the client does not support `SNI`, then it
man-in-the-middles the connection with a provided cert
to get the `Host` from the request, then continues as
described previously.

#### Whitelist

It uses a line-separated file to have a list of regexs
for the host whitelist for matching against
`<scheme>://<host>/`.

## Usage
```
# smykowski -h
Usage of smykowski:
  -certfile string
        CA certificate (default "ca.crt")
  -hostfile string
        line separated host regex whitelist (default "whitelist.lsv")
  -httpaddr string
        proxy and http listen address (default ":3129")
  -httpsaddr string
        tls listen address (default ":3128")
  -keyfile string
        CA key (default "ca.key")
  -v    should every proxy request be logged to stdout
```

#### Whitelist format
Example LSV:
```
https://(.*\.)?google\.com/$
https://(www\.)?yahoo\.com/$
```

#### Reloading whitelist
You can reload the whitelist from disk by sending a
`SIGCONT` signal to the process.
```
kill -s SIGUSR1 smykowski
```

### Potential Issues

- Whitelist cache isn't LRU so it may fill up memory
  (though it's only based on `Host` hits, so probably not
  an issue)
- Requires that non-SNI clients use a custom CA

