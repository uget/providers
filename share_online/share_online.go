package share_online

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/uget/uget/core/api"
)

type Provider struct{}

var _ api.MultiResolver = &Provider{}

func (p *Provider) Name() string {
	return "share-online.biz"
}

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, "share-online.biz") &&
		strings.HasPrefix(u.Path, "/dl/") || u.Path == "download.php" {
		return api.Multi
	}
	return api.Next
}

const linkcheck = "https://api.share-online.biz/linkcheck.php?md5=1"

func (p *Provider) ResolveMany(reqs []api.Request) ([]api.Request, error) {
	urls := make([]string, len(reqs))
	for i, req := range reqs {
		urls[i] = req.URL().String()
	}
	body := strings.Join(urls, "\n")
	resp, err := http.Post(linkcheck, "", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	c := csv.NewReader(resp.Body)
	results := make([]api.Request, len(reqs))
	for i := 0; ; i++ {
		// ID;STATUS;NAME;SIZE;MD5
		rec, err := c.Read()
		if err != nil {
			if err == io.EOF {
				return results, nil
			}
			return nil, err
		}
		size, err := strconv.Atoi(rec[3])
		if err != nil {
			return nil, err
		}
		id, md5 := rec[0], rec[4]
		u := urlFrom(id)
		if rec[1] == "OK" {
			bs, err := hex.DecodeString(md5)
			if err != nil {
				results[i] = reqs[i].Errs(u, err)
			} else {
				results[i] = reqs[i].ResolvesTo(&file{rec[2], u, bs, int64(size)})
			}
		} else {
			// "DELETED" or "NOT FOUND"
			results[i] = reqs[i].Deadend(u)
		}
	}
}

type file struct {
	name string
	u    *url.URL
	md5  []byte
	size int64
}

func (f *file) Provider() api.Provider {
	return &Provider{}
}

func (f *file) URL() *url.URL {
	return f.u
}

func (f *file) Size() int64 {
	return f.size
}

func (f *file) Name() string {
	return f.name
}

func (f *file) Checksum() ([]byte, string, hash.Hash) {
	return f.md5, "MD5", md5.New()
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("http://www.share-online.biz/dl/%s", id))
	return u
}
