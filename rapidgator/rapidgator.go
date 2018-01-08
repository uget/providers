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
	"time"

	"github.com/uget/uget/core"
	"github.com/uget/uget/core/api"
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
	expires time.Time
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

func (r Provider) Name() string {
	return "rapidgator.net"
}

func (r *Provider) CanResolve(u *url.URL) bool {
	return strings.HasSuffix(u.Host, "rapidgator.net")
}

func refreshSession(c *http.Client) error {
	mtx.Lock()
	defer mtx.Unlock()
	if session.expires.Before(time.Now()) {
		m, code, err := request(c, fmt.Sprintf(loginURL, user, password))
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("[rapidgator.net] status code %v when getting session key", code)
		}
		session.sid = m["session_id"].(string)
		session.expires = time.Now().Add(5 * time.Minute)
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

func (r *Provider) Resolve(u *url.URL) (api.File, error) {
	paths := strings.Split(u.RequestURI(), "/")[1:]
	if paths[0] != "file" {
		return nil, fmt.Errorf("url doesn't point to a file")
	}
	u = normalize(u)
	c := &http.Client{}
	// Double check -- because we want to spend as little time as possible in critical section
	if session.expires.Before(time.Now()) {
		err := refreshSession(c)
		if err != nil {
			return nil, err
		}
	}
	f, code, err := request(c, fmt.Sprintf(infoURL, session.sid, u.String()))
	if err != nil {
		return nil, err
	}
	if code != 200 {
		if code == 404 {
			return file{p: r, size: api.FileSizeOffline, url: u}, nil
		} else if code == 403 { // session expired already?
			// thread unsafe but we don't care if multiple goroutines invalidate the session
			session.expires = time.Now().Add(-100 * time.Hour)
			return r.Resolve(u)
		}
		return nil, fmt.Errorf("[rapidgator.net] status code %v", code)
	}
	size, err := strconv.ParseInt(f["size"].(string), 10, 0)
	if err != nil {
		return nil, err
	}
	return file{r, f["filename"].(string), size, f["hash"].(string), u}, nil
}

func init() {
	core.RegisterProvider(&Provider{})
}
