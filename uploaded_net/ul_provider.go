package uploaded_net

import (
	"crypto/sha1"
	"encoding/csv"
	"errors"
	"fmt"
	"hash"
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
	"github.com/uget/uget/core/action"
)

type Provider struct{}

var _ core.Accountant = Provider{}
var _ core.Authenticator = Provider{}
var _ core.Getter = Provider{}
var _ core.MultiResolver = Provider{}

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

func (c Credentials) ID() string {
	return c.Id
}

func (c Credentials) String() string {
	return fmt.Sprintf("uploaded.net<id: %s, email: %s, premium: %v, expires: %v>", c.Id, c.Email, c.Premium, c.Expires)
}

func (p Provider) NewTemplate() core.Account {
	return &Credentials{}
}

func (p Provider) Login(d *core.Downloader, manager *core.AccountManager) {
	u, _ := url.Parse("http://uploaded.net")
	var accs []Credentials
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
			} else if acc.Id != "" && acc.Password != "" {
				login(d.Client, acc.Id, acc.Password)
			} else {
				log.Warnf("[%s] Could not login with '%s' because no credentials were found", p.Name(), acc.Id)
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

func (p Provider) NewAccount(prompter core.Prompter) (core.Account, error) {
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
		c := Credentials{
			Id:          id,
			Password:    pw,
			LoginCookie: cookie.Value,
		}
		fillAccountInfo(client, &c)
		return c, nil
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

func (p Provider) CanResolve(url *url.URL) bool {
	return strings.HasSuffix(url.Host, "uploaded.net") ||
		strings.HasSuffix(url.Host, "uploaded.to") ||
		strings.HasSuffix(url.Host, "ul.to")
}

type file struct {
	id     string
	length int64
	sha1   string
	name   string
	url    *url.URL
}

var _ core.File = file{}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Filename() string {
	return f.name
}

func (f file) Length() int64 {
	return f.length
}

func (f file) Checksum() (string, string, hash.Hash) {
	return f.sha1, "SHA1", sha1.New()
}

func (p Provider) Resolve(urls []*url.URL) ([]core.File, error) {
	body := "apikey=575de523-3d0e-411a-9ebc-af9c6fff8370"
	i := 0
	for _, url := range urls {
		paths := strings.Split(url.Path, "/")[1:]
		id := ""
		if paths[0] == "file" {
			id = paths[1]
		} else if strings.HasSuffix(url.Host, "ul.to") {
			id = paths[0]
		} else if paths[0] == "f" || paths[0] == "folder" {
			return nil, fmt.Errorf("folders not supported yet")
		} else {
			return nil, fmt.Errorf("can't handle %v", url)
		}
		body += fmt.Sprintf("&id_%d=%s", i, id)
		i++
	}
	req, _ := http.NewRequest("POST", "https://uploaded.net/api/filemultiple", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	csv := csv.NewReader(resp.Body)
	csv.FieldsPerRecord = 0
	records, err := csv.ReadAll()
	if err != nil {
		return nil, err
	}
	fs := make([]core.File, 0, len(records))
	for _, record := range records {
		if record[0] == "offline" {
			fs = append(fs, file{length: -1, url: urlFrom(record[1])})
		} else if record[0] != "online" {
			return nil, fmt.Errorf("file error: %v", record[0])
		}
		if len, err := strconv.ParseInt(record[2], 10, 0); err == nil {
			id := record[1]
			fs = append(fs, file{id, len, record[3], record[4], urlFrom(id)})
		}
	}
	return fs, nil
}

func urlFrom(id string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("https://uploaded.net/file/%s", id))
	return u
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
	core.RegisterProvider(Provider{})
}
