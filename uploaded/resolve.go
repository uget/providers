package uploaded

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"

	"github.com/PuerkitoBio/goquery"
	log "github.com/Sirupsen/logrus"
	api "github.com/uget/uget/core/api"
)

func (p *Provider) CanResolve(u *url.URL) api.Resolvability {
	if strings.HasSuffix(u.Host, "uploaded.net") {
		// Folder - evaluate by itself
		if strings.HasPrefix(u.Path, "/f/") || strings.HasPrefix(u.Path, "/folder/") {
			return api.Single
		}
	}
	// prefix /dl/ in path is direct file -- leave it to the basic provider.
	if strings.HasSuffix(u.Host, "uploaded.net") && !strings.HasPrefix(u.Path, "/dl/") ||
		strings.HasSuffix(u.Host, "uploaded.to") ||
		strings.HasSuffix(u.Host, "ul.to") {
		return api.Multi
	}
	return api.Next
}

// ResolveOne -- must be folder!
func (p *Provider) ResolveOne(r api.Request) ([]api.Request, error) {
	resp, err := p.client.Get(r.URL().String())
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, err
	}
	list := doc.Find("#fileList tbody > tr")
	urls := make([]*url.URL, 0, list.Size())
	list.Each(func(i int, sel *goquery.Selection) {
		id, ok := sel.Attr("id")
		if ok {
			urls = append(urls, urlFrom(id))
		}
	})
	log.Debugf("uploaded#ResolveOne: resolved more links: %v", len(urls))
	return r.Bundles(urls), nil
}

func (p *Provider) ResolveMany(rs []api.Request) ([]api.Request, error) {
	logrus.Debugf("uploaded#ResolveMany: %v", len(rs))
	body := fmt.Sprintf("apikey=%s", apikey)
	i := 0
	for _, r := range rs {
		paths := strings.Split(r.URL().RequestURI(), "/")[1:]
		id := ""
		if paths[0] == "file" {
			id = paths[1]
		} else if strings.HasSuffix(r.URL().Host, "ul.to") {
			id = paths[0]
		} else if paths[0] == "f" || paths[0] == "folder" {
			// should be handled by ResolveOne
			panic("folder in ResolveMany")
		} else {
			return nil, fmt.Errorf("can't handle %v", r.URL())
		}
		body += fmt.Sprintf("&id_%d=%s", i, id)
		i++
	}
	req, _ := http.NewRequest("POST", "https://uploaded.net/api/filemultiple", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	c := csv.NewReader(resp.Body)
	requests := make([]api.Request, len(rs))
	for i := 0; ; i++ {
		record, err := c.Read()
		if err != nil {
			if err == io.EOF {
				return requests, nil
			}
			return nil, fmt.Errorf("csv: %v", err)
		}
		var f api.File
		if record[0] == "offline" {
			requests[i] = rs[i].Deadend()
			continue
		} else if record[0] != "online" {
			return nil, fmt.Errorf("uploaded.net returned response: %v", record[0])
		}
		if len, err := strconv.ParseInt(record[2], 10, 0); err == nil {
			id := record[1]
			f = file{p, id, len, record[3], record[4], urlFrom(id)}
		} else {
			return nil, err
		}
		requests[i] = rs[i].ResolvesTo(f)
	}
}
