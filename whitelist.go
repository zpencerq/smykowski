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

type WhitelistManager struct {
	sync.RWMutex
	entries  []Entry
	cache    map[string]bool
	filename string
}

func NewWhitelistManager(filename string) (*WhitelistManager, error) {
	twm := &WhitelistManager{
		filename: filename,
		cache:    make(map[string]bool),
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

func (wm *WhitelistManager) CheckTlsHost(host string) bool {
	return wm.CheckString("https://" + host + "/")
}

func (wm *WhitelistManager) CheckString(str string) bool {
	wm.RLock()
	defer wm.RUnlock()

	if _, present := wm.cache[str]; present {
		return true
	}

	for _, entry := range wm.entries {
		if entry.MatchesString(str) {
			wm.cache[str] = true
			return true
		}
	}

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

		if ok := wm.Check(req.URL); ok {
			log.Printf("IP %s visited - %v", ip, req.URL)
			return req, nil
		}
		log.Printf("IP %s was blocked visiting - %v", ip, req.URL)

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
	if os.IsNotExist(err) {
		tmp, err = os.Create(wm.filename)
	}
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
