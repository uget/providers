package uploaded_net

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/uget/uget/core"
)

func (p *Provider) CanResolve(url *url.URL) bool {
	// prefix /dl/ in path is direct file -- leave it to the basic provider.
	return strings.HasSuffix(url.Host, "uploaded.net") && !strings.HasPrefix(url.Path, "/dl/") ||
		strings.HasSuffix(url.Host, "uploaded.to") ||
		strings.HasSuffix(url.Host, "ul.to")
}

func (p *Provider) Resolve(urls []*url.URL) ([]core.File, error) {
	body := fmt.Sprintf("apikey=%s", apikey)
	i := 0
	for _, url := range urls {
		paths := strings.Split(url.RequestURI(), "/")[1:]
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
	records, err := csv.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %v", err)
	}
	fs := make([]core.File, 0, len(records))
	for _, record := range records {
		if record[0] == "offline" {
			fs = append(fs, file{p: p, length: -1, url: urlFrom(record[1])})
		} else if record[0] != "online" {
			return nil, fmt.Errorf("file error: %v", record[0])
		}
		if len, err := strconv.ParseInt(record[2], 10, 0); err == nil {
			id := record[1]
			fs = append(fs, file{p, id, len, record[3], record[4], urlFrom(id)})
		}
	}
	return fs, nil
}
