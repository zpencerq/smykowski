package main

import (
	"net/http"
	"testing"

	"github.com/zpencerq/goproxy"
)

func AssertDidNotPanic(t *testing.T, name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s did panic!", name)
		}
	}()

	f()
}

func DummyReq(t *testing.T) *http.Request {
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.RemoteAddr = "127.0.0.1:57862"

	return req
}

func HandlerInput(r *http.Request) (*http.Request, *goproxy.ProxyCtx) {
	return r, &goproxy.ProxyCtx{Req: r}
}

func TestReqHandler(t *testing.T) {
	source := NewMemoryWhitelistLoader(
		[]Entry{NewEntry("https://google.com")},
	)
	wm, _ := NewWhitelistManager(source)

	r := DummyReq(t)
	r.URL = nil

	handler := wm.ReqHandler()

	run := func() { handler(HandlerInput(r)) }

	AssertDidNotPanic(t, "ReqHandler with nil URL", run)

	_, resp := handler(HandlerInput(r))
	if resp.StatusCode != 400 {
		t.Errorf("nil URL response is not 400: %d", resp.StatusCode)
	}
}
