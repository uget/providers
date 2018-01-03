package oboom

import (
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/uget/uget/core"
)

// Validations

var _ core.MultiResolver = &Provider{}

var _ core.File = file{}

type Provider struct{}

type file struct {
	p    *Provider
	name string
	size int64
	url  *url.URL
}

const (
	infoURL  = "https://api.oboom.com/1.0/info"
	loginURL = "https://www.oboom.com/1.0/guestsession"
)

var mtx = sync.Mutex{}

var session = struct {
	id      string
	expires time.Time
}{}

func (f file) Provider() core.Provider {
	return f.p
}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Name() string {
	return f.name
}

func (f file) Size() int64 {
	return f.size
}

func (f file) Checksum() (string, string, hash.Hash) {
	return "", "", nil
}

func (p *Provider) Name() string {
	return "oboom.net"
}

func (p *Provider) CanResolve(u *url.URL) bool {
	return strings.HasSuffix(u.Host, "oboom.com")
}

func refreshSession(c *http.Client) error {
	mtx.Lock()
	defer mtx.Unlock()
	if session.expires.Before(time.Now()) {
		req, _ := http.NewRequest("GET", loginURL, nil)
		sid, code, err := request(c, req)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("[oboom.net] status code %v when getting session key", code)
		}
		session.id = sid.(string)
		session.expires = time.Now().Add(23 * time.Hour)
	}
	return nil
}

func request(c *http.Client, req *http.Request) (interface{}, int, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, -1, err
	}
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	var arr []interface{}
	err = json.Unmarshal(rbody, &arr)
	if err != nil {
		return nil, -1, err
	}
	code := int(arr[0].(float64))
	return arr[1], code, nil
}

func (p *Provider) Resolve(urls []*url.URL) ([]core.File, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}
	c := &http.Client{}
	// Double check -- because we want to spend as little time as possible in critical section
	if session.expires.Before(time.Now()) {
		err := refreshSession(c)
		if err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(urls))
	for _, u := range urls {
		paths := strings.Split(u.RequestURI(), "/")[1:]
		ids = append(ids, paths[0])
	}
	body := fmt.Sprintf("token=%s&items=%s&http_errors=0", session.id, strings.Join(ids, ","))
	// use POST to not run any risk of 414 Request-URI Too Long
	req, _ := http.NewRequest("POST", infoURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	i, code, err := request(c, req)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		if code == 403 { // session expired already?
			// thread unsafe but we don't care if multiple goroutines invalidate the session
			session.expires = time.Now().Add(-100 * time.Hour)
			return p.Resolve(urls)
		}
		return nil, fmt.Errorf("[oboom.net] status code %v", code)
	}
	arr := i.([]interface{})
	fs := make([]core.File, 0, len(arr))
	for _, m := range arr {
		record := m.(map[string]interface{})
		var f core.File
		u := urlFrom(record["id"].(string))
		if record["state"] != "online" {
			f = file{p: p, size: -1, url: u}
		} else {
			f = file{
				p:    p,
				size: int64(record["size"].(float64)),
				name: record["name"].(string),
				url:  u,
			}
		}
		fs = append(fs, f)
	}
	return fs, nil
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://oboom.com/%s", id))
	return u
}

func init() {
	core.RegisterProvider(&Provider{})
}
