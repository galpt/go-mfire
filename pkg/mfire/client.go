package mfire

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Client handles HTTP requests to MangaFire and parsing.
type Client struct {
	http *http.Client
}

// NewClient returns a client with a reasonable timeout and TLS settings that
// tolerate typical scraping setups.
func NewClient() *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	// include a cookie jar to preserve session cookies between requests;
	// some sites set a session cookie on the home page which later requests
	// expect.
	jar, _ := cookiejar.New(nil)
	return &Client{
		http: &http.Client{Transport: tr, Timeout: 15 * time.Second, Jar: jar},
	}
}

// fetchVrfWithBrowser launches a headless Chrome instance, loads the site,
// injects the search query into the page and listens for the outgoing AJAX
// request that contains a server-generated `vrf` token. Returns the token or
// an error if not found within timeout.
func fetchVrfWithBrowser(q string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Exec allocator with common flags. Requires Chrome/Chromium on the host.
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	defer allocCancel()

	// Create a chromedp context and silence chromedp's internal debug logs
	// which may include unknown/new CDP enum values depending on the
	// installed Chrome version. We don't need verbose cdproto logging here.
	cctx, cancel2 := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(string, ...interface{}) {}))
	defer cancel2()

	var vrf string
	done := make(chan struct{})

	chromedp.ListenTarget(cctx, func(ev interface{}) {
		if e, ok := ev.(*network.EventRequestWillBeSent); ok {
			// Look for requests that either hit ajax/search or include a vrf param
			if strings.Contains(e.Request.URL, "ajax/manga/search") || strings.Contains(e.Request.URL, "/filter?") {
				if u, err := url.Parse(e.Request.URL); err == nil {
					if v := u.Query().Get("vrf"); v != "" {
						vrf = v
						select {
						case <-done:
						default:
							close(done)
						}
					}
				}
			}
		}
	})

	// Enable network domain so we receive network events
	if err := chromedp.Run(cctx, network.Enable()); err != nil {
		return "", err
	}

	// Load the page and trigger the site's search UI via injected JS.
	js := fmt.Sprintf(`(function(){
		const el = document.querySelector('.search-inner input[name=keyword]');
		if (!el) return false;
		el.value = %q;
		el.dispatchEvent(new Event('keyup'));
		return true;
	})();`, q)

	if err := chromedp.Run(cctx,
		chromedp.Navigate("https://mangafire.to/home"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Evaluate(js, nil),
	); err != nil {
		return "", err
	}

	select {
	case <-done:
		return vrf, nil
	case <-ctx.Done():
		return "", fmt.Errorf("timeout waiting for vrf: %w", ctx.Err())
	}
}

func (c *Client) fetchDocument(rawurl string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", rawurl, nil)
	if err != nil {
		return nil, err
	}
	// Use a common browser User-Agent to reduce the chance of blocking.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://mangafire.to/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

// FetchHome lists manga titles found on the home page, limited to 'limit'.
func (c *Client) FetchHome(limit int) ([]Manga, error) {
	doc, err := c.fetchDocument("https://mangafire.to/home")
	if err != nil {
		return nil, err
	}
	mangas := make([]Manga, 0, limit)
	doc.Find(".original.card-lg .unit .inner").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(mangas) >= limit {
			return false
		}
		a := s.Find(".info > a").First()
		title := a.Text()
		href, _ := a.Attr("href")
		cover, _ := s.Find("img").Attr("src")
		// Normalize url if relative
		u := href
		if parsed, err := url.Parse(href); err == nil && !parsed.IsAbs() {
			u = "https://mangafire.to" + href
		}
		mangas = append(mangas, Manga{Title: title, Url: u, Cover: cover})
		return true
	})
	return mangas, nil
}

// Search performs a site search using the required vrf parameter and returns up to limit results.
func (c *Client) Search(query string, limit int) ([]Manga, error) {
	qTrim := strings.TrimSpace(query)

	// Preflight: fetch the filter page to populate cookies and any session state.
	// Many clients (Kotatsu/Mihon) request /filter before performing searches
	// which sets cookies the server expects for subsequent calls.
	_, _ = c.fetchDocument("https://mangafire.to/filter")

	// Build keyword query similar to the reference implementation: split on
	// whitespace, URL-encode each part, then join with '+' so phrases like
	// "chainsaw man" become "chainsaw+man" with each part percent-encoded.
	parts := strings.Fields(qTrim)
	for i := range parts {
		parts[i] = url.QueryEscape(parts[i])
	}
	encodedQuery := strings.Join(parts, "+")

	vrf, err := GenerateVrf(qTrim)
	if err != nil {
		return nil, err
	}

	searchURL := "https://mangafire.to/filter?keyword=" + encodedQuery + "&vrf=" + url.QueryEscape(vrf)

	// Build request manually so we can set Referer to the filter page (the
	// Kotlin implementation uses a Referer header pointing at the domain or
	// filter page via an interceptor). Some servers expect the Referer to be
	// the search/filter UI.
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://mangafire.to/filter")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// If we hit a 403, try a headless-browser fallback to obtain a
		// server-generated vrf token (the site computes vrf client-side via JS).
		if resp.StatusCode == 403 {
			fmt.Printf("search: initial request returned 403 — attempting headless-browser vrf fallback\n")
			browserVrf, berr := fetchVrfWithBrowser(qTrim, 20*time.Second)
			if berr == nil && browserVrf != "" {
				// retry the search using the browser-provided vrf
				searchURL = "https://mangafire.to/filter?keyword=" + encodedQuery + "&vrf=" + url.QueryEscape(browserVrf)
				req2, rerr := http.NewRequest("GET", searchURL, nil)
				if rerr != nil {
					return nil, rerr
				}
				req2.Header = req.Header.Clone()
				resp2, rerr := c.http.Do(req2)
				if rerr != nil {
					return nil, rerr
				}
				defer resp2.Body.Close()
				if resp2.StatusCode >= 400 {
					return nil, fmt.Errorf("bad status: %s", resp2.Status)
				}
				doc, err := goquery.NewDocumentFromReader(resp2.Body)
				if err != nil {
					return nil, err
				}
				// continue parsing below using doc
				mangas := make([]Manga, 0, limit)
				doc.Find(".original.card-lg .unit .inner").EachWithBreak(func(i int, s *goquery.Selection) bool {
					if len(mangas) >= limit {
						return false
					}
					a := s.Find(".info > a").First()
					title := a.Text()
					href, _ := a.Attr("href")
					cover, _ := s.Find("img").Attr("src")
					u := href
					if parsed, err := url.Parse(href); err == nil && !parsed.IsAbs() {
						u = "https://mangafire.to" + href
					}
					mangas = append(mangas, Manga{Title: title, Url: u, Cover: cover})
					return true
				})
				return mangas, nil
			}
		}
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	mangas := make([]Manga, 0, limit)
	doc.Find(".original.card-lg .unit .inner").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(mangas) >= limit {
			return false
		}
		a := s.Find(".info > a").First()
		title := a.Text()
		href, _ := a.Attr("href")
		cover, _ := s.Find("img").Attr("src")
		u := href
		if parsed, err := url.Parse(href); err == nil && !parsed.IsAbs() {
			u = "https://mangafire.to" + href
		}
		mangas = append(mangas, Manga{Title: title, Url: u, Cover: cover})
		return true
	})
	return mangas, nil
}
