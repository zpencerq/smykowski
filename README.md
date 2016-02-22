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

It doesn’t do MITM, instead it acts like a TLS vhost
to utilize SNI to get the `Host` from the client. Then,
it annotates the request with a `CONNECT` and hands
it off to `goproxy` (after checking the Host to make
sure it fits in the whitelist).

#### Whitelist

It uses a line-separated file to have a list of regexs
for the host whitelist for matching against
`<scheme>://<host>/`.

## Usage
```
# smykowski -h
Usage of smykowski:
  -hostfile string
        line separated host regex whitelist (default "whitelist.lsv")
  -httpaddr string
        proxy http listen address (default ":3129")
  -httpsaddr string
        proxy https listen address (default ":3128")
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
kill -s SIGCONT smykowski
```

### Potential Issues

- Whitelist cache isn't LRU so it may fill up memory
  (though it's only based on `Host` hits, so probably not
  an issue)
- Does not support non-SNI (old) clients

