package uploaded_net

import (
	"crypto/sha1"
	"hash"
	"net/url"

	"github.com/uget/uget/core"
)

var _ core.File = file{}

type file struct {
	id     string
	length int64
	sha1   string
	name   string
	url    *url.URL
}

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
	return f.sha1, "SHA1", sha1.New()
}