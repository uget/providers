package rapidgator

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/uget/uget/core/api"
)

// Validations

var _ api.SingleResolver = &Provider{}

var _ api.File = file{}

type Provider struct{}

type file struct {
	p        *Provider
	filename string
	size     int64
	md5      []byte
	url      *url.URL
}

const (
	infoURL  = "https://rapidgator.net/api/file/info?sid=%s&url=%s"
	loginURL = "https://rapidgator.net/api/user/login?username=%s&password=%s"
	user     = "uget@dispostable.com"
	password = "public8)"
)

var mtx = sync.Mutex{}

var session = struct {
	sid     string
	expires int64
}{}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Provider() api.Provider {
	return f.p
}

func (f file) Name() string {
	return f.filename
}

func (f file) Size() int64 {
	return f.size
}

func (f file) Checksum() ([]byte, string, hash.Hash) {
	return f.md5, "MD5", md5.New()
}

func (p *Provider) Name() string {
	return "rapidgator.net"
}

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, "rapidgator.net") {
		return api.Single
	}
	return api.Next
}

func refreshSession() error {
	mtx.Lock()
	defer mtx.Unlock()
	if atomic.LoadInt64(&session.expires) < time.Now().Unix() {
		raw, code, err := request(fmt.Sprintf(loginURL, user, password))
		if err != nil {
			return err
		}
		var j struct {
			SessionID string `json:"session_id"`
		}
		err = json.Unmarshal(*raw, &j)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("status code %v when getting session key", code)
		}
		session.sid = j.SessionID
		atomic.StoreInt64(&session.expires, time.Now().Unix()+5*60)
	}
	return nil
}

func request(url string) (*json.RawMessage, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, -1, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if len(body) == 0 || body[0] == '<' {
		return nil, -1, errors.New("got non-JSON response")
	}
	var j struct {
		ResponseStatus int `json:"response_status"`
		Response       *json.RawMessage
	}
	err = json.Unmarshal(body, &j)
	if err != nil {
		return nil, -1, err
	}
	return j.Response, j.ResponseStatus, nil
}

func (p *Provider) ResolveOne(req api.Request) ([]api.Request, error) {
	paths := strings.Split(req.URL().RequestURI(), "/")[1:]
	normalized, _ := url.Parse(fmt.Sprintf("https://rapidgator.net/file/%s", paths[1]))
	if paths[0] != "file" {
		return req.Errs(normalized, errors.New("url doesn't point to a file")).Wrap(), nil
	}
	// Double check -- because we want to spend as little time as possible in critical section
	if atomic.LoadInt64(&session.expires) < time.Now().Unix() {
		err := refreshSession()
		if err != nil {
			return req.Errs(normalized, err).Wrap(), nil
		}
	}
	raw, code, err := request(fmt.Sprintf(infoURL, session.sid, normalized.String()))
	if err != nil {
		return req.Errs(normalized, err).Wrap(), nil
	}
	if code != 200 {
		if code == 404 {
			return req.Deadend(normalized).Wrap(), nil
		} else if code == 403 { // session expired already?
			atomic.StoreInt64(&session.expires, time.Now().Add(-100*time.Hour).Unix())
			return p.ResolveOne(req)
		}
		return req.Errs(normalized, fmt.Errorf("status code %v", code)).Wrap(), nil
	}
	var j struct {
		Filename string
		Size     string
		Hash     string
	}
	err = json.Unmarshal(*raw, &j)
	if err != nil {
		return req.Errs(normalized, err).Wrap(), nil
	}
	size, err := strconv.ParseInt(j.Size, 10, 0)
	if err != nil {
		return req.Errs(normalized, err).Wrap(), nil
	}
	bs, err := hex.DecodeString(j.Hash)
	if err != nil {
		return req.Errs(normalized, err).Wrap(), nil
	}
	resolved := file{p, j.Filename, size, bs, normalized}
	return req.ResolvesTo(resolved).Wrap(), nil
}
