package uploaded_net

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/action"
)

var _ core.Accountant = uploaded{}
var _ core.Authenticator = uploaded{}
var _ core.Getter = uploaded{}
var _ core.MultiResolver = uploaded{}

type uploaded struct{}

const apikey = "575de523-3d0e-411a-9ebc-af9c6fff8370"

func (p uploaded) Name() string {
	return "uploaded.net"
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://uploaded.net/file/%s", id))
	return u
}

func (p uploaded) Action(r *http.Response, d *core.Downloader) *action.Action {
	if !strings.HasSuffix(r.Request.URL.Host, "uploaded.net") {
		return action.Next()
	}
	if r.StatusCode != http.StatusOK {
		return action.Deadend()
	}
	if strings.HasPrefix(r.Request.URL.RequestURI(), "/dl/") {
		return action.Goal()
	}
	if strings.HasPrefix(r.Request.URL.RequestURI(), "/file/") {
		doc, _ := goquery.NewDocumentFromResponse(r)
		val, ok := doc.Find("#download.center form").First().Attr("action")
		if ok {
			if val == "register" {
				return action.Deadend()
			}
			u, _ := url.Parse(val)
			return action.Redirect(u)
		}
	} else if strings.HasPrefix(r.Request.URL.RequestURI(), "/folder/") {
		doc, _ := goquery.NewDocumentFromResponse(r)
		list := doc.Find("#fileList tbody > tr")
		links := make([]*url.URL, 0, list.Size())
		list.Each(func(i int, sel *goquery.Selection) {
			attr, ok := sel.Attr("id")
			if ok {
				links = append(links, urlFrom(attr))
			}
		})
		log.Debugf("[uploaded.net] Resolved more links: %v", len(links))
		return action.Bundle(links)
	}
	log.Errorf("[uploaded.net] Don't know what to do with response from: %v", r.Request.URL)
	return action.Deadend()
}

func init() {
	core.RegisterProvider(uploaded{})
}
