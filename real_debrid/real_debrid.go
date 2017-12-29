package real_debrid

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/action"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Provider struct{}
type Credentials struct {
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Points   int       `json:"fidelity"`
	Premium  bool      `json:"premium"`
	Expires  time.Time `json:"expires_at"`
	ApiToken string    `json:"apitoken" sensitive:"true"`
}

func init() {
	core.RegisterProvider(Provider{})
}

func (c Credentials) String() string {
	return fmt.Sprintf("real-debrid.com<username: %s, email: %s, premium: %v, fidelity: %v>", c.Username, c.Email, c.Premium, c.Points)
}

const apiBase = "https://api.real-debrid.com/rest/1.0"

func (p Provider) Name() string {
	return "real-debrid.com"
}

func (p Provider) Action(r *http.Response, d *core.Downloader) *action.Action {
	return action.Next()
}

/**
 * Deprecated: use NewAccount instead.
 */
func (p Provider) manager() *core.AccountManager {
	return core.AccountManagerFor("", p)
}

func (p Provider) NewAccount(prompter core.Prompter) (string, interface{}, error) {
	fields := []core.Field{
		{"apitoken", "Token (collect from https://real-debrid.com/apitoken)", true, ""},
	}
	tok := prompter.Get(fields)["apitoken"]
	client := &http.Client{}
	if req, err := http.NewRequest("GET", strings.Join([]string{apiBase, "user"}, "/"), nil); err != nil {
		return "", nil, err
	} else {
		req.Header.Add("Authorization", strings.Join([]string{"Bearer", tok}, " "))
		if resp, err := client.Do(req); err == nil {
			if resp.StatusCode != 200 {
				return "", nil, errors.New(resp.Status)
			}
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return "", nil, err
			}
			m := map[string]interface{}{}
			err = json.Unmarshal([]byte(body), &m)
			if err != nil {
				return "", nil, err
			}
			c := &Credentials{
				m["username"].(string),
				m["email"].(string),
				int(m["points"].(float64)),
				m["type"] == "premium",
				time.Now().Add(time.Duration(m["premium"].(float64)) * time.Second),
				tok,
			}
			return c.Username, c, nil
		} else {
			return "", nil, err
		}
	}
}

func (p Provider) NewTemplate() interface{} {
	return &Credentials{}
}
