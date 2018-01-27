package zippyshare

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/uget/uget/core/api"
)

type Provider struct{}

var _ api.SingleResolver = &Provider{}

func (p *Provider) Name() string {
	return "zippyshare.com"
}

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, ".zippyshare.com") && strings.HasPrefix(u.Path, "/v/") {
		return api.Single
	}
	return api.Next
}

func (p *Provider) ResolveOne(req api.Request) ([]api.Request, error) {
	resp, err := http.Get(req.URL().String())
	if err != nil {
		return nil, err
	}
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	r := regexp.MustCompile(`document\.getElementById\('dlbutton'\)\.href = "/(p)?d/(.*?)/" \+ \((\d+) \% (\d+) \+ \d+ \% (\d+)\) \+ "/(.*?)";`)
	matches := r.FindStringSubmatch(string(bs))
	logrus.Debugf("[zippyshare] match: %v", matches[0])
	ref, bases, mod1s, mod2s, name := matches[2], matches[3], matches[4], matches[5], matches[6]
	base, err := strconv.Atoi(bases)
	if err != nil {
		return nil, err
	}
	mod1, err := strconv.Atoi(mod1s)
	if err != nil {
		return nil, err
	}
	mod2, err := strconv.Atoi(mod2s)
	if err != nil {
		return nil, err
	}
	u := new(url.URL)
	*u = *req.URL()
	u.Path = "/d/" + ref + "/" + strconv.Itoa(base%mod1+base%mod2) + "/" + name
	if matches[1] == "p" {
		// for /pd/ links, we need to "HEAD" the /pd/ URL once to activate the ticket
		if _, err := http.Head(strings.Replace(u.String(), ".com/d/", ".com/pd/", 1)); err != nil {
			return nil, err
		}
	}
	return req.Yields(u).Wrap(), nil
}
