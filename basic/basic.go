package basic

import (
	"fmt"
	"hash"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	api "github.com/uget/uget/core/api"
)

type Provider struct {
	client *http.Client
	accts  []api.Account
}

var _ api.Accountant = &Provider{}
var _ api.Retriever = &Provider{}
var _ api.SingleResolver = &Provider{}
var _ api.Configured = &Provider{}

func (p *Provider) Name() string {
	return "basic"
}

func (p *Provider) Configure(c *api.Config) {
	p.client = &http.Client{}
	p.accts = c.Accounts
}

func (p *Provider) Retrieve(f api.File) (*http.Request, error) {
	req, err := http.NewRequest("GET", f.URL().String(), nil)
	if err != nil {
		return nil, err
	}
	p.login(req)
	return req, nil
}

type Account struct {
	Username string
	Password string
	Host     string
}

func (a *Account) ID() string {
	return a.Username + "@" + a.Host
}

func (a *Account) String() string {
	return a.ID()
}

func (p *Provider) NewTemplate() api.Account {
	return &Account{}
}

func (p *Provider) NewAccount(pr api.Prompter) (api.Account, error) {
	fields := []api.Field{
		{"username", "username", false, ""},
		{"password", "password", true, ""},
		{"host", "host", false, ""},
	}
	m, err := pr.Get(fields)
	if err != nil {
		return nil, err
	}
	return &Account{
		Username: m["username"],
		Password: m["password"],
		Host:     m["host"],
	}, nil
}

type file struct {
	p      *Provider
	name   string
	length int64
	url    *url.URL
}

var _ api.File = file{}

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
	return f.length
}

func (f file) Checksum() ([]byte, string, hash.Hash) {
	return nil, "", nil
}

func (p *Provider) CanRetrieve(api.File) uint {
	return 1
}

func (p *Provider) CanResolve(*url.URL) api.Resolvability {
	return api.Single
}

func (p *Provider) ResolveOne(r api.Request) ([]api.Request, error) {
	logrus.Debugf("basic#ResolveOne: %v", r.URL())
	if !r.URL().IsAbs() {
		return nil, fmt.Errorf("non-absolute URL")
	}
	req, _ := http.NewRequest("HEAD", r.URL().String(), nil)
	p.login(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return r.Deadend(nil).Wrap(), nil // 404 => file is offline
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}
	f := file{url: r.URL()}
	if resp.ContentLength == -1 {
		f.length = api.FileSizeUnknown
	} else {
		f.length = resp.ContentLength
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			f.name = params["filename"]
		}
	} else {
		paths := strings.Split(r.URL().RequestURI(), "/")
		rawName := paths[len(paths)-1]
		name, err := url.PathUnescape(rawName)
		if err != nil {
			name = rawName
		}
		if name == "" {
			f.name = "index.html"
		} else {
			f.name = name
		}
	}
	return r.ResolvesTo(f).Wrap(), nil
}

func (p *Provider) login(req *http.Request) {
	for _, acc := range p.accts {
		account := acc.(*Account)
		if strings.HasSuffix(req.URL.Host, account.Host) {
			req.SetBasicAuth(account.Username, account.Password)
		}
	}

}
