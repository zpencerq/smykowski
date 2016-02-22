package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/zpencerq/goproxy"
)

func SetupProxy(proxy *goproxy.ProxyHttpServer) {
	proxy.Verbose = *verbose
	if proxy.Verbose {
		log.Printf("Server starting up! - configured to listen on http interface %s and https interface %s", *http_addr, *https_addr)
	}

	proxy.NonproxyHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host == "" {
			fmt.Fprintln(w, "Cannot handle requests without Host header, e.g., HTTP 1.0")
			return
		}
		req.URL.Scheme = "http"
		req.URL.Host = req.Host
		proxy.ServeHTTP(w, req)
	})

	proxy.OnRequest().DoFunc(wm.ReqHandler())

	proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		ip, _, err := net.SplitHostPort(ctx.Req.RemoteAddr)
		if err != nil {
			panic(fmt.Sprintf("userip: %q is not IP:port", ctx.Req.RemoteAddr))
		}
		userIP := net.ParseIP(ip)
		if userIP == nil {
			panic(fmt.Sprintf("userip: %q is not IP", ip))
		}

		log.Printf("Handled connect from ip - %s - for host %s", ip, host)
		if err != nil {
			log.Printf("Error creating URL for host %s", host)
		}

		if wm.CheckTlsHost(ctx.Req.URL.String()) {
			// don't tear down the SSL session
			return &goproxy.ConnectAction{
				Action: goproxy.ConnectAccept,
			}, host
		} else {
			return goproxy.RejectConnect, host
		}
	})
}

type DumbResponseWriter struct {
	net.Conn
}

func (dumb DumbResponseWriter) Header() http.Header {
	panic("Header() should not be called on this ResponseWriter")
}

var OK = []byte("HTTP/1.0 200 OK\r\n\r\n")

func (dumb DumbResponseWriter) Write(buf []byte) (int, error) {
	if bytes.Equal(buf, OK) {
		return len(buf), nil // throw away the HTTP OK response from the faux CONNECT request
	}
	return dumb.Conn.Write(buf)
}

func (dumb DumbResponseWriter) WriteHeader(code int) {
	panic("WriteHeader() should not be called on this ResponseWriter")
}

func (dumb DumbResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return dumb, bufio.NewReadWriter(bufio.NewReader(dumb), bufio.NewWriter(dumb)), nil
}
