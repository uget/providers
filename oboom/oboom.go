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

	"github.com/Sirupsen/logrus"
	api "github.com/uget/uget/core/api"
)

// Validations

var _ api.MultiResolver = &Provider{}
var _ api.SingleResolver = &Provider{}

var _ api.File = file{}

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

func (f file) Provider() api.Provider {
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

func (f file) Checksum() ([]byte, string, hash.Hash) {
	return nil, "", nil
}

func (p *Provider) Name() string {
	return "oboom.net"
}

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, "oboom.com") {
		if strings.HasPrefix(u.Path, "/folder/") {
			return api.Single
		}
		return api.Multi
	}
	return api.Next
}

func refreshSession() error {
	mtx.Lock()
	defer mtx.Unlock()
	if session.expires.Before(time.Now()) {
		req, _ := http.NewRequest("GET", loginURL, nil)
		raw, code, err := request(req)
		if err != nil {
			return err
		}
		if code != 200 {
			return fmt.Errorf("[oboom.net] status code %v when getting session key", code)
		}
		var sid string
		err = json.Unmarshal(*raw, &sid)
		if err != nil {
			return err
		}
		session.id = sid
		session.expires = time.Now().Add(23 * time.Hour)
	}
	return nil
}

var client = &http.Client{}

func request(req *http.Request) (*json.RawMessage, int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, -1, err
	}
	rbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	var arr []*json.RawMessage
	err = json.Unmarshal(rbody, &arr)
	if err != nil {
		return nil, -1, err
	}
	var code int
	err = json.Unmarshal(*arr[0], &code)
	if err != nil {
		return nil, -1, err
	}
	return arr[1], code, nil
}

func (p *Provider) ResolveOne(req api.Request) ([]api.Request, error) {
	return nil, api.ErrTODO
}

func (p *Provider) ResolveMany(reqs []api.Request) ([]api.Request, error) {
	if len(reqs) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}
	// Double check -- because we want to spend as little time as possible in critical section
	if session.expires.Before(time.Now()) {
		err := refreshSession()
		if err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(reqs))
	idToIndex := make(map[string]int)
	for i, req := range reqs {
		paths := strings.Split(req.URL().RequestURI(), "/")[1:]
		ids = append(ids, paths[0])
		idToIndex[paths[0]] = i
	}
	body := fmt.Sprintf("token=%s&items=%s&http_errors=0", session.id, strings.Join(ids, ","))
	logrus.Debugf("oboom#ResolveMany: %v", body)
	// use POST to not run any risk of 414 Request-URI Too Long
	req, _ := http.NewRequest("POST", infoURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	raw, code, err := request(req)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		if code == 403 { // session expired already?
			// thread unsafe but we don't care if multiple goroutines invalidate the session
			session.expires = time.Now().Add(-100 * time.Hour)
			return p.ResolveMany(reqs)
		}
		return nil, fmt.Errorf("[oboom.net] status code %v", code)
	}
	var files []struct {
		ID    string
		State string
		Type  string
		Size  int64
		Name  string
	}
	err = json.Unmarshal(*raw, &files)
	if err != nil {
		return nil, err
	}
	requests := make([]api.Request, len(files))
	for _, f := range files {
		i := idToIndex[f.ID]
		if f.State != "online" {
			requests[i] = reqs[i].Deadend(nil)
		} else if f.Type == "folder" {
			folder, _ := url.Parse("https://oboom.com/folder/" + f.ID)
			requests[i] = reqs[i].Yields(folder)
		} else {
			u := urlFrom(f.ID)
			apiFile := file{
				p:    p,
				size: f.Size,
				name: f.Name,
				url:  u,
			}
			requests[i] = reqs[i].ResolvesTo(apiFile)
		}
	}
	return requests, nil
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://oboom.com/%s", id))
	return u
}
