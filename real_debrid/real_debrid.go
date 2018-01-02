package real_debrid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/uget/uget/core"
)

type credentials struct {
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Points   int       `json:"fidelity"`
	Premium  bool      `json:"premium"`
	Expires  time.Time `json:"expires_at"`
	APIToken string    `json:"apitoken" sensitive:"true"`
}

var _ core.Account = credentials{} // verify that credentials implements interface

type realDebrid struct{}

var _ core.Accountant = realDebrid{} // verify that realDebrid implements interface

func (c credentials) ID() string {
	return c.Username
}

func (c credentials) String() string {
	return fmt.Sprintf("real-debrid.com<username: %s, email: %s, premium: %v, fidelity: %v>", c.Username, c.Email, c.Premium, c.Points)
}

const apiBase = "https://api.real-debrid.com/rest/1.0"

func (p realDebrid) Name() string {
	return "real-debrid.com"
}

func (p realDebrid) NewAccount(prompter core.Prompter) (core.Account, error) {
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

func (p realDebrid) NewTemplate() core.Account {
	return &credentials{}
}

func init() {
	core.RegisterProvider(realDebrid{})
}
