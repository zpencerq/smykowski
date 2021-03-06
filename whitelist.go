package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/zpencerq/goproxy"
)

type WhitelistLoader interface {
	Load(adder func(entry Entry)) error
}

type MemoryWhitelistLoader struct {
	entries []Entry
}

func NewMemoryWhitelistLoader(entries []Entry) *MemoryWhitelistLoader {
	return &MemoryWhitelistLoader{entries}
}

func (mws *MemoryWhitelistLoader) Load(adder func(entry Entry)) error {
	for _, entry := range mws.entries {
		adder(entry)
	}

	return nil
}

type FileWhitelistLoader struct {
	filename string
}

func NewFileWhitelistLoader(filename string) (*FileWhitelistLoader, error) {
	fws := &FileWhitelistLoader{filename: filename}

	if err := fws.Load(func(entry Entry) {}); err != nil {
		return nil, err
	}

	return fws, nil
}

func (fws *FileWhitelistLoader) Load(adder func(entry Entry)) error {
	tmp, err := os.Open(fws.filename)
	if err != nil {
		return err
	}

	defer func() {
		err := tmp.Close()
		if err != nil {
			log.Printf("Error closing file - %v", err)
		}
	}()

	scanner := bufio.NewScanner(tmp)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		adder(NewEntry(scanner.Text()))
		log.Printf("Added: %v", scanner.Text())
	}

	return nil
}

type WhitelistManager struct {
	sync.RWMutex
	entries []Entry
	cache   map[string]bool
	loader  WhitelistLoader

	Tracker Tracker
	Verbose bool
}

func NewWhitelistManager(loader WhitelistLoader) (*WhitelistManager, error) {
	twm := &WhitelistManager{
		loader:  loader,
		cache:   make(map[string]bool),
		Tracker: NewNoopTracker(),
	}
	err = twm.load()
	return twm, err
}

func (wm *WhitelistManager) Refresh() error {
	wm.Lock()
	defer wm.Unlock()

	oldEntries := make([]Entry, len(wm.entries))
	copy(oldEntries, wm.entries)
	wm.entries = nil
	wm.cache = make(map[string]bool)

	err := wm.load()
	if err != nil {
		wm.entries = oldEntries
	}

	return err
}

func (wm *WhitelistManager) Add(entry Entry) {
	wm.Lock()
	defer wm.Unlock()

	wm.add(entry)
}

func (wm *WhitelistManager) Size() int {
	wm.RLock()
	defer wm.RUnlock()

	return len(wm.entries)
}

func (wm *WhitelistManager) Check(URL *url.URL) bool {
	return wm.CheckString(URL.String())
}

func (wm *WhitelistManager) CheckHttpHost(host string) bool {
	return wm.CheckString("http://" + host)
}

func (wm *WhitelistManager) CheckTlsHost(host string) bool {
	return wm.CheckString("https://" + host)
}

func (wm *WhitelistManager) trackAllow(str string) error {
	return wm.Tracker.Track(NewEvent(
		"smykowski.allow",
		map[string]interface{}{
			"Value": 1,
			"Type":  Counter,
			"Tags": map[string]string{
				"Value": str,
			},
		}))
}

func (wm *WhitelistManager) trackBlock(str string) error {
	return wm.Tracker.Track(NewEvent(
		"smykowski.block",
		map[string]interface{}{
			"Value": 1,
			"Type":  Counter,
			"Tags": map[string]string{
				"Value": str,
			},
		}))
}

func (wm *WhitelistManager) CheckString(str string) bool {
	wm.RLock()
	defer wm.RUnlock()

	if _, present := wm.cache[str]; present {
		defer wm.trackAllow(str)
		return true
	}

	for _, entry := range wm.entries {
		if entry.MatchesString(str) {
			wm.cache[str] = true
			defer wm.trackAllow(str)
			return true
		}
	}

	defer wm.trackBlock(str)

	return false
}

func BadRequestResponse(ctx *goproxy.ProxyCtx) *http.Response {
	return &http.Response{
		StatusCode: 400,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Request:    ctx.Req,
		Header:     http.Header{"Cache-Control": []string{"no-cache"}},
	}
}

func (wm *WhitelistManager) ReqHandler() goproxy.FuncReqHandler {
	return func(r *http.Request, ctx *goproxy.ProxyCtx) (req *http.Request, resp *http.Response) {
		resp = nil

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("Recovered from panic: %v", rec)
				resp = &http.Response{
					StatusCode: 500,
					ProtoMajor: 1,
					ProtoMinor: 1,
					Request:    ctx.Req,
					Header:     http.Header{"Cache-Control": []string{"no-cache"}},
				}
			}
		}()

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.Printf("userip: %q is not IP:port", r.RemoteAddr)
			resp = BadRequestResponse(ctx)
			return
		}
		userIP := net.ParseIP(ip)
		if userIP == nil {
			log.Printf("userip: %q is not IP", ip)
			resp = BadRequestResponse(ctx)
			return
		}
		if r.URL == nil {
			log.Printf("Bad Request: URL is nil (from %q)", userIP)
			resp = BadRequestResponse(ctx)
			return
		}

		if r.URL.Host == "" { // this is a mitm'd request
			r.URL.Host = r.Host
			host := r.URL.Host

			if ok := wm.CheckTlsHost(host); ok {
				if wm.Verbose {
					log.Printf("IP %s visited - %v", ip, host)
				}
				return
			}
		}
		host := r.URL.Host
		hostaddr, port, err := net.SplitHostPort(host)
		if err != nil { // host didn't have a port
			hostaddr = host
		}
		if port == "443" {
			if ok := wm.CheckTlsHost(hostaddr); ok {
				if wm.Verbose {
					log.Printf("IP %s visited - %v", ip, hostaddr)
				}
				return
			}
		}

		if ok := wm.CheckHttpHost(hostaddr); ok {
			if wm.Verbose {
				log.Printf("IP %s visited - %v", ip, hostaddr)
			}
			return
		}
		log.Printf("IP %s was blocked visiting - %v", ip, hostaddr)

		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprint("<html><body>Requested destination not in whitelist</body></html>"))

		resp = &http.Response{
			StatusCode:    403,
			ProtoMajor:    1,
			ProtoMinor:    1,
			Request:       ctx.Req,
			Header:        http.Header{"Cache-Control": []string{"no-cache"}},
			Body:          ioutil.NopCloser(&buf),
			ContentLength: int64(buf.Len()),
		}

		return
	}
}

func (wm *WhitelistManager) load() error {
	return wm.loader.Load(wm.add)
}

func (wm *WhitelistManager) add(entry Entry) {
	wm.entries = append(wm.entries, entry)
}
