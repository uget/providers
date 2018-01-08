package uploaded

import (
	"crypto/sha1"
	"hash"
	"net/url"

	api "github.com/uget/uget/core/api"
)

var _ api.File = file{}

type file struct {
	p      *Provider
	id     string
	length int64
	sha1   string
	name   string
	url    *url.URL
}

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

func (f file) Checksum() (string, string, hash.Hash) {
	return f.sha1, "SHA1", sha1.New()
}
