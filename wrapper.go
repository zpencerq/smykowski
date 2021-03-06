package main

import (
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
		NewEvent("smykowski.serve",
			map[string]interface{}{
				"Tags": map[string]string{
					"Host":     host,
					"Protocol": protocol,
				},
				"Url": r.URL.Path,
			}),
		time.Now())

	proxy.ServeHTTP(w, r)
}
