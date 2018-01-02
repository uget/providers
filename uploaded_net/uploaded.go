package uploaded_net

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/uget/uget/core"
	"github.com/uget/uget/utils"
)

var _ core.Accountant = &Provider{}
var _ core.Configured = &Provider{}
var _ core.Retriever = &Provider{}
var _ core.MultiResolver = &Provider{}

type Provider struct {
	client *http.Client
	mgr    *core.AccountManager
	once   *utils.Once
}

const apikey = "575de523-3d0e-411a-9ebc-af9c6fff8370"

func (p *Provider) Name() string {
	return "uploaded.net"
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://uploaded.net/file/%s", id))
	return u
}

func (p *Provider) Configure(c *core.Config) {
	p.mgr = c.AccountManager
	p.once = &utils.Once{}
}

func (p *Provider) CanRetrieve(f core.File) uint {
	if p.CanResolve(f.URL()) {
		return 100
	}
	return 0
}

func (p *Provider) Retrieve(f core.File) (io.ReadCloser, error) {
	if err := p.once.Do(func() error { return p.login() }); err != nil {
		return nil, err
	}
	resp, err := p.client.Get(f.URL().String())
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(resp.Request.URL.RequestURI(), "/dl/") {
		return resp.Body, nil
	}
	doc, _ := goquery.NewDocumentFromResponse(resp)
	val, ok := doc.Find("#download.center form").First().Attr("action")
	if !ok {
		return nil, fmt.Errorf("couldn't find the download action")
	}
	if val == "register" {
		return nil, fmt.Errorf("account expired")
	}
	goal, err := p.client.Get(val)
	if err != nil {
		return nil, err
	}
	return goal.Body, nil
}

func init() {
	jar, _ := cookiejar.New(nil)
	core.RegisterProvider(&Provider{client: &http.Client{Jar: jar}})
}
