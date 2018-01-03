package basic

import (
	"hash"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/uget/uget/core"
)

type Provider struct {
	client *http.Client
}

var _ core.Retriever = &Provider{}
var _ core.SingleResolver = &Provider{}

func (p *Provider) Name() string {
	return "basic"
}

func (p *Provider) Retrieve(f core.File) (*http.Request, error) {
	return http.NewRequest("GET", f.URL().String(), nil)
}

type file struct {
	p      *Provider
	name   string
	length int64
	url    *url.URL
}

var _ core.File = file{}

func (f file) Provider() core.Provider {
	return f.p
}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Name() string {
	return f.name

}

func (f file) Length() int64 {
	return f.length
}

func (f file) Checksum() (string, string, hash.Hash) {
	return "", "", nil
}

func (p *Provider) CanRetrieve(core.File) uint {
	return 1
}

func (p *Provider) CanResolve(*url.URL) bool {
	return true
}

func (p *Provider) Resolve(u *url.URL) (core.File, error) {
	if !u.IsAbs() {
	}
	c := &http.Client{}
	req, _ := http.NewRequest("HEAD", u.String(), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	f := file{length: resp.ContentLength, url: u}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			f.name = params["filename"]
		}
	} else {
		paths := strings.Split(u.RequestURI(), "/")
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
	return f, nil
}

func init() {
	core.RegisterProvider(&Provider{
		client: &http.Client{},
	})
}
