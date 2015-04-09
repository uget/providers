package uploaded_net

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/cihub/seelog"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/action"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

type Provider struct{}

func (p Provider) Name() string {
	return "uploaded.net"
}

type Credentials struct {
	LoginCookie string `json:"login_cookie"`
}

func (p Provider) Login(d *core.Downloader) {
	// u, _ := url.Parse("http://uploaded.net")

	// TODO: Read sensitive user information from somewhere and set client cookies.
	// e.g.:
	// credentials := Credentials{}
	// accounts := account.Get(p.Name(), &credentials)
}

func login(id string, pw string) (*http.Cookie, error) {
	reader := strings.NewReader(fmt.Sprintf("id=%s&pw=%s", id, pw))
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	u, _ := url.Parse("https://uploaded.net/io/login")
	req, _ := http.NewRequest("POST", u.String(), reader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "login" {
			return cookie, nil
		}
	}
	return nil, errors.New("Could not find login cookie in response headers.")
}

func (p Provider) AddAccount(prompter core.Prompter) {
	prompter.Define(&core.Field{"id", "username", false, ""})
	prompter.Define(&core.Field{"password", "password", true, ""})
	values := prompter.Get()
	id := values["id"]
	pw := values["password"]

	// do the request
	cookie, err := login(id, pw)
	_ = cookie
	if err != nil {
		prompter.Error(err.Error())
	} else {
		// account.Add(p.Name(), id, Credentials{cookie.Value})
		prompter.Success()
	}
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
