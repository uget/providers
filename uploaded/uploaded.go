package uploaded

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	// "github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	api "github.com/uget/uget/core/api"
	"github.com/uget/uget/utils"
)

const apikey = "575de523-3d0e-411a-9ebc-af9c6fff8370"

var _ api.Accountant = &Provider{}
var _ api.Configured = &Provider{}
var _ api.Retriever = &Provider{}
var _ api.MultiResolver = &Provider{}

type Provider struct {
	client *http.Client
	accts  []api.Account
	once   *utils.Once
}

func (p *Provider) Name() string {
	return "uploaded.net"
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("http://uploaded.net/file/%s", id))
	return u
}

func (p *Provider) Configure(c *api.Config) {
	p.accts = c.Accounts
	p.once = &utils.Once{}
	jar, _ := cookiejar.New(nil)
	p.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.HasPrefix(req.URL.Path, "/dl/") {
				return http.ErrUseLastResponse
			}
			return nil
		},
		Jar: jar,
	}
}

func (p *Provider) CanRetrieve(f api.File) uint {
	if p.CanResolve(f.URL()) != api.Next {
		return 100
	}
	return 0
}

func (p *Provider) Retrieve(f api.File) (*http.Request, error) {
	if err := p.once.Do(func() error { return p.login() }); err != nil {
		return nil, err
	}
	resp, err := p.client.Get(f.URL().String())
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(resp.Status, "3") {
		loc, err := resp.Location()
		if err != nil {
			return nil, err
		}
		log.Debugf("[uploaded.net] Redirect to %v", loc)
		return http.NewRequest("GET", loc.String(), nil)
	}
	doc, _ := goquery.NewDocumentFromResponse(resp)
	val, ok := doc.Find("#download.center form").First().Attr("action")
	if !ok {
		return nil, fmt.Errorf("couldn't find the download action")
	}
	if val == "register" {
		return nil, fmt.Errorf("account expired")
	}
	return http.NewRequest("GET", val, nil)
}
