package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/inconshreveable/go-vhost"
	"github.com/zpencerq/goproxy"
)

var (
	verbose    *bool
	http_addr  *string
	https_addr *string
	host_file  *string
	err        error

	proxy *goproxy.ProxyHttpServer
	wm    *WhitelistManager
)

func init() {
	verbose = flag.Bool("v", false, "should every proxy request be logged to stdout")
	http_addr = flag.String("httpaddr", ":3129", "proxy http listen address")
	https_addr = flag.String("httpsaddr", ":3128", "proxy https listen address")
	host_file = flag.String("hostfile", "whitelist.lsv", "line separated host regex whitelist")
	flag.Parse()

	wm, err = NewWhitelistManager(*host_file)
	if err != nil {
		panic(err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGCONT)

	go func(c chan os.Signal) {
		for {
			s := <-c
			log.Printf("Caught signal - %v: refreshing.", s)
			wm.Refresh()
		}
	}(sigc)

	proxy = goproxy.NewProxyHttpServer()
}

func main() {
	SetupProxy(proxy)

	go func() {
		log.Fatalln(http.ListenAndServe(*http_addr, proxy))
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
			if tlsConn.Host() == "" {
				log.Printf("Cannot support non-SNI enabled clients")
				c.Close()
				return
			}

			if !wm.CheckTlsHost(tlsConn.Host()) {
				log.Printf("Denied %v before CONNECT", tlsConn.Host())
				c.Close()
				return
			}

			connectReq := &http.Request{
				RemoteAddr: c.RemoteAddr().String(),
				Method:     "CONNECT",
				URL: &url.URL{
					Opaque: tlsConn.Host(),
					Host:   net.JoinHostPort(tlsConn.Host(), "443"),
				},
				Host:   tlsConn.Host(),
				Header: make(http.Header),
			}
			resp := DumbResponseWriter{tlsConn}
			proxy.ServeHTTP(resp, connectReq)
		}(c)
	}
}
