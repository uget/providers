package rapidgator

import (
	"crypto/md5"
	"encoding/json"
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
	md5      string
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

func (f file) Checksum() (string, string, hash.Hash) {
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

func refreshSession(c *http.Client) error {
	mtx.Lock()
	defer mtx.Unlock()
	if atomic.LoadInt64(&session.expires) < time.Now().Unix() {
		m, code, err := request(c, fmt.Sprintf(loginURL, user, password))
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("[rapidgator.net] status code %v when getting session key", code)
		}
		session.sid = m["session_id"].(string)
		atomic.StoreInt64(&session.expires, time.Now().Unix()+5*60)
	}
	return nil
}

func request(c *http.Client, url string) (map[string]interface{}, int, error) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, -1, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	m := map[string]interface{}{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		return nil, -1, err
	}
	code := int(m["response_status"].(float64))
	if code == 200 {
		m = m["response"].(map[string]interface{})
	}
	return m, code, nil
}

func normalize(u *url.URL) *url.URL {
	htmlExt := ".html"
	for strings.HasSuffix(u.Path, htmlExt) {
		u2, _ := url.Parse("/")
		*u2 = *u
		u2.Path = u2.Path[0 : len(u2.Path)-len(htmlExt)]
		u = u2
	}
	return u
}

func (p *Provider) ResolveOne(req api.Request) ([]api.Request, error) {
	paths := strings.Split(req.URL().RequestURI(), "/")[1:]
	if paths[0] != "file" {
		return nil, fmt.Errorf("url doesn't point to a file")
	}
	c := &http.Client{}
	// Double check -- because we want to spend as little time as possible in critical section
	if atomic.LoadInt64(&session.expires) < time.Now().Unix() {
		err := refreshSession(c)
		if err != nil {
			return nil, err
		}
	}
	f, code, err := request(c, fmt.Sprintf(infoURL, session.sid, req.URL().String()))
	if err != nil {
		return nil, err
	}
	if code != 200 {
		if code == 404 {
			return req.Deadend(normalize(req.URL())).Wrap(), nil
		} else if code == 403 { // session expired already?
			atomic.StoreInt64(&session.expires, time.Now().Add(-100*time.Hour).Unix())
			return p.ResolveOne(req)
		}
		return nil, fmt.Errorf("[rapidgator.net] status code %v", code)
	}
	size, err := strconv.ParseInt(f["size"].(string), 10, 0)
	if err != nil {
		return nil, err
	}
	resolved := file{p, f["filename"].(string), size, f["hash"].(string), normalize(req.URL())}
	return req.ResolvesTo(resolved).Wrap(), nil
}
