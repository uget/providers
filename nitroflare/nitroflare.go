package nitroflare

import (
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/uget/uget/core/api"
)

type Provider struct{}

var _ api.MultiResolver = &Provider{}

func (p *Provider) Name() string {
	return "nitroflare.com"
}

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, "nitroflare.com") && strings.HasPrefix(u.Path, "/view/") {
		return api.Multi
	}
	return api.Next
}

const linkcheck = "https://nitroflare.com/api/v2/getFileInfo?files="

func (p *Provider) ResolveMany(reqs []api.Request) ([]api.Request, error) {
	ids := make([]string, len(reqs))
	for i, req := range reqs {
		id := strings.Split(req.URL().Path, "/")[2]
		ids[i] = id
	}
	resp, err := http.Get(linkcheck + strings.Join(ids, ","))
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(resp.Body)
	var j struct {
		Type   string
		Result *json.RawMessage
	}
	err = decoder.Decode(&j)
	if err != nil {
		return nil, err
	}
	if j.Type == "success" {
		var result struct {
			Files map[string]struct {
				Status string
				Name   string
				Size   string
				// URL    string
				// UploadDate string
				// PremiumOnly bool
				// ? MD5         string
				// ? Password    string
			}
		}
		err = json.Unmarshal(*j.Result, &result)
		if err != nil {
			return nil, err
		}
		results := make([]api.Request, len(result.Files))
		for i, id := range ids { // range over ids to maintain order
			u := urlFrom(id)
			f := result.Files[id]
			if f.Status == "online" {
				size, err := strconv.Atoi(f.Size)
				if err != nil {
					results[i] = reqs[i].Errs(u, err)
				} else {
					results[i] = reqs[i].ResolvesTo(&file{f.Name, u, int64(size)})
				}
			} else {
				results[i] = reqs[i].Deadend(u)
			}
		}
		return results, nil
	}
	return nil, fmt.Errorf("got error '%v' from nitroflare.com API", j.Type)
}

type file struct {
	name string
	u    *url.URL
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
	return nil, "", nil
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://www.nitroflare.com/view/%s", id))
	return u
}
