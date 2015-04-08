package uploaded_net

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/cihub/seelog"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/action"
	"net/http"
	"net/url"
	"strings"
)

type Provider struct{}

func (p Provider) Name() string {
	return "uploaded.net"
}

func (p Provider) Login(d *core.Downloader) {
	// u, _ := url.Parse("http://uploaded.net")
	// TODO: Read sensitive user information from somewhere and set client cookies.
}

func (p Provider) Action(r *http.Response, d *core.Downloader) *action.Action {
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
			u, _ := url.Parse(val)
			return action.Redirect(u)
		}
	} else if strings.HasPrefix(r.Request.URL.RequestURI(), "/folder/") {
		doc, _ := goquery.NewDocumentFromResponse(r)
		list := doc.Find("#fileList tbody > tr")
		links := make([]string, 0, list.Size())
		list.Each(func(i int, sel *goquery.Selection) {
			attr, ok := sel.Attr("id")
			if ok {
				links = append(links, fmt.Sprintf("http://uploaded.net/file/%s", attr))
			}
		})
		log.Debugf("Resolved more links: %v", links)
		return action.Bundle(links)
	}
	log.Errorf("[uploaded.net] Don't know what to do with response from: %v", r.Request.URL)
	return action.Deadend()
}

func init() {
	core.RegisterProvider(Provider{})
}
