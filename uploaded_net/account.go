package uploaded_net

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	"github.com/uget/uget/core"
)

type credentials struct {
	Name        string    `json:"id"`
	Password    string    `json:"password" sensitive:"true"`
	Email       string    `json:"email"`
	Premium     bool      `json:"premium"`
	Expires     time.Time `json:"expires_at"`
	LoginCookie string    `json:"login_cookie" sensitive:"true"`
}

func (c credentials) ID() string {
	return c.Name
}

func (c credentials) String() string {
	return fmt.Sprintf("uploaded.net<id: %s, email: %s, premium: %v, expires: %v>", c.Name, c.Email, c.Premium, c.Expires)
}

func (p uploaded) NewTemplate() core.Account {
	return &credentials{}
}

func (p uploaded) Login(d *core.Downloader, manager *core.AccountManager) {
	u, _ := url.Parse("http://uploaded.net")
	var accs []credentials
	manager.Accounts(&accs)
	for _, acc := range accs {
		if acc.Premium {
			if acc.LoginCookie != "" {
				d.Client.Jar.SetCookies(u, []*http.Cookie{
					{
						Name:   "login",
						Value:  acc.LoginCookie,
						Domain: "uploaded.net",
						Path:   "/",
					},
				})
			} else if acc.Name != "" && acc.Password != "" {
				login(d.Client, acc.Name, acc.Password)
			} else {
				log.Warnf("[%s] Could not login with '%s' because no credentials were found", p.Name(), acc.Name)
				continue
			}
			break
		}
	}
}

func login(client *http.Client, id string, pw string) (*http.Cookie, error) {
	reader := strings.NewReader(fmt.Sprintf("id=%s&pw=%s", id, pw))
	req, _ := http.NewRequest("POST", "https://uploaded.net/io/login", reader)
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
	return nil, errors.New("[uploaded.net] Could not find login cookie in response headers.")
}

func (p uploaded) NewAccount(prompter core.Prompter) (core.Account, error) {
	fields := []core.Field{
		{"username", "username", false, ""},
		{"password", "password", true, ""},
	}
	values := prompter.Get(fields)
	id := values["username"]
	pw := values["password"]
	// do the request
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	cookie, err := login(client, id, pw)
	_ = cookie
	if err != nil {
		return nil, err
	} else {
		c := credentials{
			Name:        id,
			Password:    pw,
			LoginCookie: cookie.Value,
		}
		fillAccountInfo(client, &c)
		return c, nil
	}
}

func fillAccountInfo(client *http.Client, c *credentials) {
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
