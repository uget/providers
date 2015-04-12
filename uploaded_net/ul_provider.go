package uploaded_net

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	log "github.com/cihub/seelog"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/account"
	"github.com/uget/uget/core/action"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Provider struct{}

func (p Provider) Name() string {
	return "uploaded.net"
}

type Credentials struct {
	Id          string    `json:"id"`
	Password    string    `json:"password" sensitive:"true"`
	Email       string    `json:"email"`
	Premium     bool      `json:"premium"`
	Expires     time.Time `json:"expires_at"`
	LoginCookie string    `json:"login_cookie" sensitive:"true"`
}

func (c Credentials) String() string {
	return fmt.Sprintf("uploaded.net<id: %s, email: %s, premium: %v, expires: %v>", c.Id, c.Email, c.Premium, c.Expires)
}

func (p Provider) NewTemplate() interface{} {
	return &Credentials{}
}

func (p Provider) manager() *account.Manager {
	return account.ManagerFor("", p)
}

func (p Provider) Login(d *core.Downloader) {
	u, _ := url.Parse("http://uploaded.net")
	var accs []Credentials
	p.manager().Accounts(&accs)
	for _, acc := range accs {
		if acc.Premium {
			d.Client.Jar.SetCookies(u, []*http.Cookie{
				{
					Name:   "login",
					Value:  acc.LoginCookie,
					Domain: "uploaded.net",
					Path:   "/",
				},
			})
			break
		}
	}
}

func login(client *http.Client, id string, pw string) (*http.Cookie, error) {
	reader := strings.NewReader(fmt.Sprintf("id=%s&pw=%s", id, pw))
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
	fields := []core.Field{
		{"id", "username", false, ""},
		{"password", "password", true, ""},
	}
	values := prompter.Get(fields)
	id := values["id"]
	pw := values["password"]
	// do the request
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	cookie, err := login(client, id, pw)
	_ = cookie
	if err != nil {
		prompter.Error(err.Error())
	} else {
		c := Credentials{
			Id:          id,
			Password:    pw,
			LoginCookie: cookie.Value,
		}
		fillAccountInfo(client, &c)
		p.manager().AddAccount(c.Id, c)
		prompter.Success()
	}
}

func fillAccountInfo(client *http.Client, c *Credentials) {
	request, _ := http.NewRequest("GET", "https://uploaded.net", nil)
	resp, err := client.Do(request)
	if err != nil {
		return
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return
	}
	s := doc.Find("#account table.data").First()
	email := s.Find("#chMail").Get(0).Attr[1].Val
	c.Email = email
	defpre := s.Find("a[href=register]").First().Children().Text()
	c.Premium = defpre == "Premium"
	duration := s.Find("tr:contains('Duration') th:not(#chAlias)").Text()
	matches := regexp.MustCompile(`(?i)(\d+) weeks? (\d+) days? and (\d+) hours?`).FindStringSubmatch(duration)
	weeks, err1 := strconv.Atoi(matches[1])
	days, err2 := strconv.Atoi(matches[2])
	hours, err3 := strconv.Atoi(matches[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	c.Expires = time.Now().Add(time.Duration(hours+(days+weeks*7)*24) * time.Hour)
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
			if val == "register" {
				return action.Deadend()
			}
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
		log.Debugf("Resolved more links: %v", len(links))
		return action.Bundle(links)
	}
	log.Errorf("[uploaded.net] Don't know what to do with response from: %v", r.Request.URL)
	return action.Deadend()
}

func init() {
	core.RegisterProvider(Provider{})
}
