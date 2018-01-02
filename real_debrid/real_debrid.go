package real_debrid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/uget/providers/oboom"
	"github.com/uget/providers/rapidgator"
	"github.com/uget/providers/uploaded_net"
	"github.com/uget/uget/core"
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

var _ core.Account = &credentials{} // verify that credentials implements interface

var _ core.Accountant = &Provider{} // verify that realDebrid implements interface

var _ core.Configured = &Provider{} // verify that provider implements interface

type Provider struct {
	mgr *core.AccountManager
}

func (c credentials) ID() string {
	return c.Username
}

func (c credentials) String() string {
	return fmt.Sprintf("real-debrid.com<username: %s, email: %s, premium: %v, fidelity: %v>", c.Username, c.Email, c.Premium, c.Points)
}

func (p *Provider) Configure(c *core.Config) {
	p.mgr = c.AccountManager
}

func (p *Provider) Name() string {
	return "real-debrid.com"
}

func (p *Provider) CanRetrieve(f core.File) uint {
	if (&uploaded_net.Provider{}).CanResolve(f.URL()) ||
		(&rapidgator.Provider{}).CanResolve(f.URL()) ||
		(&oboom.Provider{}).CanResolve(f.URL()) {
		selected, _ := p.mgr.SelectedAccount()
		if selected != nil {
			acc := selected.(*credentials)
			if acc.Premium && acc.Expires.Before(time.Now()) {
				// Prefer this as it has unlimited traffic
				return 500
			}
		}
	}
	return 0
}

func (p *Provider) Retrieve(f core.File) (io.ReadCloser, error) {
	selected, _ := p.mgr.SelectedAccount()
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
	download, err := c.Get(m["download"].(string))
	if err != nil {
		return nil, err
	}
	return download.Body, nil
}

func (p *Provider) NewAccount(prompter core.Prompter) (core.Account, error) {
	fields := []core.Field{
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

func (p *Provider) NewTemplate() core.Account {
	return &credentials{}
}

func init() {
	core.RegisterProvider(&Provider{})
}
