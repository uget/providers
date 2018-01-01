package basic

import (
	"hash"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/uget/uget/core"
	"github.com/uget/uget/core/action"
)

type basic struct{}

var _ core.Getter = basic{}
var _ core.SingleResolver = basic{}

func (p basic) Name() string {
	return "default"
}

func (p basic) Action(r *http.Response, d *core.Downloader) *action.Action {
	if r.StatusCode != http.StatusOK {
		return action.Deadend()
	}
	// TODO: Make action dependent on content type?
	// ensure underlying body is indeed a file, and not a html page / etc.
	return action.Goal()
}

type file struct {
	name   string
	length int64
	url    *url.URL
}

var _ core.File = file{}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Filename() string {
	return f.name

}

func (f file) Length() int64 {
	return f.length
}

func (f file) Checksum() (string, string, hash.Hash) {
	return "", "", nil
}

func (p basic) CanResolve(*url.URL) bool {
	return true
}

func (p basic) Resolve(u *url.URL) (core.File, error) {
	if !u.IsAbs() {
	}
	c := &http.Client{}
	req, _ := http.NewRequest("HEAD", u.String(), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	disposition := resp.Header.Get("Content-Disposition")
	f := file{length: resp.ContentLength, url: u}
	arr := regexp.MustCompile(`filename="(.*?)"`).FindStringSubmatch(disposition)
	if len(arr) > 1 {
		f.name = arr[1]
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
	core.RegisterProvider(basic{})
}
