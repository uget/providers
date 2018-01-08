package real_debrid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/uget/providers/oboom"
	"github.com/uget/providers/rapidgator"
	"github.com/uget/providers/uploaded"
	"github.com/uget/uget/core"
	api "github.com/uget/uget/core/api"
)

const apiBase = "https://api.real-debrid.com/rest/1.0"

type credentials struct {
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Points   int       `json:"fidelity"`
	Premium  bool      `json:"premium"`
	Expires  time.Time `json:"expires_at"`
	APIToken string    `json:"apitoken" sensitive:"true"`
}

var _ api.Account = &credentials{} // verify that credentials implements interface

var _ api.Accountant = &Provider{} // verify that realDebrid implements interface

var _ api.Configured = &Provider{} // verify that provider implements interface

var _ api.Retriever = &Provider{} // verify that provider implements interface

type Provider struct {
	accts []api.Account
}

func (c credentials) ID() string {
	return c.Username
}

func (c credentials) String() string {
	return fmt.Sprintf("real-debrid.com<username: %s, email: %s, premium: %v, fidelity: %v>", c.Username, c.Email, c.Premium, c.Points)
}

func (p *Provider) Configure(c *api.Config) {
	p.accts = c.Accounts
}

func (p *Provider) Name() string {
	return "real-debrid.com"
}

func (p *Provider) CanRetrieve(f api.File) uint {
	if (&uploaded.Provider{}).CanResolve(f.URL()) ||
		(&rapidgator.Provider{}).CanResolve(f.URL()) ||
		(&oboom.Provider{}).CanResolve(f.URL()) {
		logrus.Debugf("[real-debrid.com] checking accounts for candidate '%v'", f.URL())
		for _, acc := range p.accts {
			account := acc.(credentials)
			if account.Premium && account.Expires.After(time.Now()) {
				logrus.Debugf("[real-debrid.com] selected account %v", account.Username)
				return 500
			}
		}
	}
	return 0
}

func (p *Provider) Retrieve(f api.File) (*http.Request, error) {
	var selected api.Account
	for _, acc := range p.accts {
		if acc.(credentials).Premium {
			selected = acc
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no account exists")
	}
	acc := selected.(*credentials)
	c := &http.Client{}
	data := fmt.Sprintf("link=%s", url.QueryEscape(f.URL().String()))
	req, _ := http.NewRequest("POST", apiBase+"/unrestrict/link", strings.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+acc.APIToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("[real-debrid.com] json: %v", string(body))
	url, ok := m["download"].(string)
	if !ok {
		es := "missing download ticket"
		code, ok := m["error_code"].(float64)
		if ok {
			es += fmt.Sprintf(" (%v)", errorCodes[int(code)])
		} else {
			es += " (unknown error)"
		}
		return nil, fmt.Errorf(es)
	}
	i := strings.LastIndexByte(url, '/')
	if strings.Contains(url[0:i], m["id"].(string)) {
		url = url[0:i]
	}
	return http.NewRequest("GET", url, nil)
}

func (p *Provider) NewAccount(prompter api.Prompter) (api.Account, error) {
	fields := []api.Field{
		{"apitoken", "Token (collect from https://real-debrid.com/apitoken)", true, ""},
	}
	vals, err := prompter.Get(fields)
	if err != nil {
		return nil, err
	}
	tok := vals["apitoken"]
	client := &http.Client{}
	if req, err := http.NewRequest("GET", strings.Join([]string{apiBase, "user"}, "/"), nil); err != nil {
		return nil, err
	} else {
		req.Header.Add("Authorization", strings.Join([]string{"Bearer", tok}, " "))
		if resp, err := client.Do(req); err == nil {
			if resp.StatusCode != 200 {
				return nil, errors.New(resp.Status)
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			m := map[string]interface{}{}
			err = json.Unmarshal([]byte(body), &m)
			if err != nil {
				return nil, err
			}
			c := &credentials{
				m["username"].(string),
				m["email"].(string),
				int(m["points"].(float64)),
				m["type"] == "premium",
				time.Now().Add(time.Duration(m["premium"].(float64)) * time.Second),
				tok,
			}
			return c, nil
		} else {
			return nil, err
		}
	}
}

func (p *Provider) NewTemplate() api.Account {
	return &credentials{}
}

func init() {
	core.RegisterProvider(&Provider{})
}
