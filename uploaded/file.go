package uploaded

import (
	"crypto/sha1"
	"hash"
	"net/url"

	api "github.com/uget/uget/core/api"
)

var _ api.File = file{}

type file struct {
	size int64
	sha1 []byte
	name string
	url  *url.URL
}

func (f file) Provider() api.Provider {
	return &Provider{}
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
	return f.sha1, "SHA1", sha1.New()
}
