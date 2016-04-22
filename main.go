package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexcesaro/statsd"
	"github.com/inconshreveable/go-vhost"
	"github.com/zpencerq/goproxy"
)

var (
	verbose     *bool
	http_addr   *string
	https_addr  *string
	host_file   *string
	cert_file   *string
	key_file    *string
	statsd_host *string
	err         error

	tracker Tracker
	proxy   *goproxy.ProxyHttpServer
	wrapper *ProxyHttpServerWrapper
	cert    tls.Certificate
	wm      *WhitelistManager
)

func init() {
	verbose = flag.Bool("v", false, "should every proxy request be logged to stdout")
	http_addr = flag.String("httpaddr", ":3128", "proxy and http listen address")
	https_addr = flag.String("httpsaddr", ":3129", "tls listen address")
	host_file = flag.String("hostfile", "whitelist.lsv", "line separated host regex whitelist")
	cert_file = flag.String("certfile", "ca.crt", "CA certificate")
	key_file = flag.String("keyfile", "ca.key", "CA key")
	statsd_host = flag.String("statsdhost", "localhost:8125", "StatsD host (e.g. localhost:8125)")
	flag.Parse()

	statsd_client, err := NewStatsdClient(
		statsd.Address(*statsd_host),
		statsd.Prefix("smykowski."),
		statsd.TagsFormat(statsd.InfluxDB),
	)
	if nil != err {
		log.Println(err)
	}

	tracker = NewStatsdTracker(statsd_client)

	wm, err = NewWhitelistManager(*host_file)
	wm.Verbose = *verbose
	wm.Tracker = tracker
	if err != nil {
		panic(err)
	}

	cert, err = tls.LoadX509KeyPair(*cert_file, *key_file)
	if err != nil {
		log.Printf("Unable to load certificate - %v", err)
		log.Printf("Using default goproxy certificate")
		cert = goproxy.GoproxyCa
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGUSR1)

	go func(c chan os.Signal) {
		for {
			s := <-c
			log.Printf("Caught signal - %v: refreshing.", s)
			wm.Refresh()
		}
	}(sigc)

	proxy = goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose

	wrapper = NewProxyWrapper(proxy)
	wrapper.Tracker = tracker
}

func main() {
	SetupProxy(proxy, cert)

	go func() {
		log.Fatalln(http.ListenAndServe(*http_addr, wrapper))
	}()

	startHttpsVhost()
}

func startHttpsVhost() {
	// listen to the TLS ClientHello but make it a CONNECT request instead
	ln, err := net.Listen("tcp", *https_addr)
	if err != nil {
		log.Fatalf("Error listening for https connections - %v", err)
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Printf("Error accepting new connection - %v", err)
			continue
		}
		go func(c net.Conn) {
			tlsConn, err := vhost.TLS(c)
			if err != nil {
				log.Printf("Error accepting new connection - %v", err)
			}

			host := tlsConn.Host()
			defer func(start time.Time) {
				tracker.Track(
					NewDurationEvent(
						fmt.Sprintf("serve.https.%s", host),
						start,
						map[string]interface{}{
							"Host":     host,
							"Protocol": "https",
						}),
				)
			}(time.Now())

			if !wm.CheckTlsHost(host) && host != "" { // don't filter non-SNI clients here
				log.Printf("Denied %v before CONNECT", tlsConn.Host())
				c.Close()
				return
			}

			connectReq := &http.Request{
				RemoteAddr: c.RemoteAddr().String(),
				Method:     "CONNECT",
				URL: &url.URL{
					Opaque: host,
					Host:   host,
				},
				Host:   host,
				Header: make(http.Header),
			}
			resp := DumbResponseWriter{tlsConn}
			proxy.ServeHTTP(resp, connectReq)
		}(c)
	}
}
