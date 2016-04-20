package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/zpencerq/goproxy"
)

type ProxyHttpServerWrapper struct {
	*goproxy.ProxyHttpServer
	Tracker Tracker
}

func NewProxyWrapper(proxy *goproxy.ProxyHttpServer) *ProxyHttpServerWrapper {
	return &ProxyHttpServerWrapper{proxy, NewNoopTracker()}
}

func (wrapper *ProxyHttpServerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	protocol := "tcp"

	host, port, err := net.SplitHostPort(r.Host)
	if err == nil { // PROXIED REQUEST
		if port == "443" {
			protocol = "https"
		}
		if port == "80" {
			protocol = "http"
		}
	} else {
		protocol = "http"
		host = r.Host
	}

	defer wrapper.TrackDuration(
		NewEvent(fmt.Sprintf("serve.%s.%s", protocol, host),
			map[string]interface{}{
				"Host":     host,
				"Url":      r.URL.Path,
				"Protocol": protocol,
			}),
		time.Now())

	proxy.ServeHTTP(w, r)
}
