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
	entries  []Entry
	cache    map[string]bool
	filename string

	Tracker Tracker
	Verbose bool
}

func NewWhitelistManager(filename string) (*WhitelistManager, error) {
	twm := &WhitelistManager{
		filename: filename,
		cache:    make(map[string]bool),
		Tracker:  NewNoopTracker(),
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

func (wm *WhitelistManager) ReqHandler() goproxy.FuncReqHandler {
	return func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			panic(fmt.Sprintf("userip: %q is not IP:port", req.RemoteAddr))
		}
		userIP := net.ParseIP(ip)
		if userIP == nil {
			panic(fmt.Sprintf("userip: %q is not IP", ip))
		}

		if req.URL.Host == "" { // this is a mitm'd request
			req.URL.Host = req.Host
			host := req.URL.Host

			if ok := wm.CheckTlsHost(host); ok {
				if wm.Verbose {
					log.Printf("IP %s visited - %v", ip, host)
				}
				return req, nil
			}
		}
		host := req.URL.Host
		hostaddr, port, err := net.SplitHostPort(host)
		if err != nil { // host didn't have a port
			hostaddr = host
		}
		if port == "443" {
			if ok := wm.CheckTlsHost(hostaddr); ok {
				if wm.Verbose {
					log.Printf("IP %s visited - %v", ip, hostaddr)
				}
				return req, nil
			}
		}

		if ok := wm.CheckHttpHost(hostaddr); ok {
			if wm.Verbose {
				log.Printf("IP %s visited - %v", ip, hostaddr)
			}
			return req, nil
		}
		log.Printf("IP %s was blocked visiting - %v", ip, hostaddr)

		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprint("<html><body>Requested destination not in whitelist</body></html>"))

		return nil, &http.Response{
			StatusCode:    403,
			ProtoMajor:    1,
			ProtoMinor:    1,
			Request:       ctx.Req,
			Header:        http.Header{"Cache-Control": []string{"no-cache"}},
			Body:          ioutil.NopCloser(&buf),
			ContentLength: int64(buf.Len()),
		}
	}
}

func (wm *WhitelistManager) load() error {
	tmp, err := os.Open(wm.filename)
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
		wm.add(NewEntry(scanner.Text()))
		log.Printf("Added: %v", scanner.Text())
	}

	return nil
}

func (wm *WhitelistManager) add(entry Entry) {
	wm.entries = append(wm.entries, entry)
}
