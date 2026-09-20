// Copyright © 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package fetch retrieves desired information from the [en.]ws-tcg.com websites.
package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"image"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

const (
	// The maximum number of workers at each stage that can do tasks locally
	// (that don't have to interact with the websites).
	maxLocalWorker int = 10

	// The maximum number of workers at each stage that have to interact with the websites.
	maxScrapeWorker int = 5

	// The English site rejects non-browser user agents with a 404 from CloudFront.
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
)

type SiteLanguage language.Tag

func (s SiteLanguage) String() string {
	return language.Tag(s).String()
}

var (
	English  SiteLanguage = SiteLanguage(language.English)
	Japanese SiteLanguage = SiteLanguage(language.Japanese)
)

// ErrIncomplete is returned (wrapped) by CardsStream and friends when the
// cards that were emitted are not a complete snapshot of the site: a results
// page or card page could not be fetched, or fewer cards were found than the
// site's own result count says there should be.
var ErrIncomplete = errors.New("scrape incomplete")

type siteConfig struct {
	baseURL       string
	baseURLValues func() url.Values
	cardListURL   string
	cardSearchURL string
	// cardsPerPage is how many cards a full search results page holds.
	cardsPerPage int
	languageCode language.Tag
	// resultCountFunc extracts the total result count the site reports on a
	// search results page.
	resultCountFunc func(doc *goquery.Document) (int, error)
	// pageScanParseFunc queues the cards on one search results page and
	// returns how many cards it found there.
	pageScanParseFunc          func(task *scrapeTask, wgCardSel *sync.WaitGroup, cardSelCh chan<- cardPage, resp *http.Response) (found int)
	recentReleaseDistinguisher string
	recentReleaseExpansionFunc func(page *goquery.Selection) *url.Values
	supportTitleNumber         bool
}

var siteConfigs = map[SiteLanguage]siteConfig{
	English: {
		baseURL: "https://en.ws-tcg.com/",
		baseURLValues: func() url.Values {
			return url.Values{
				"view": {"text"},
			}
		},
		cardListURL:   "https://en.ws-tcg.com/cardlist/",
		cardSearchURL: "https://en.ws-tcg.com/cardlist/searchresults/",
		// As of 2024-9-3, there are 15 cards per "page".
		cardsPerPage: 15,
		languageCode: language.English,
		resultCountFunc: func(doc *goquery.Document) (int, error) {
			numCardsS := doc.Find(".c-search__results-item span").First().Text()
			numCardsS = strings.TrimSpace(numCardsS)
			numCardsS = strings.ReplaceAll(numCardsS, ",", "")
			numCards, err := strconv.Atoi(numCardsS)
			if err != nil {
				return 0, fmt.Errorf("couldn't parse result count %q: %w", numCardsS, err)
			}
			return numCards, nil
		},
		pageScanParseFunc: func(task *scrapeTask, wgCardSel *sync.WaitGroup, cardSelCh chan<- cardPage, resp *http.Response) (found int) {
			log := task.client.log()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				task.fail(fmt.Errorf("read results page %s: %w", resp.Request.URL, err))
				return 0
			}
			subPaths := englishListingCardLinks(body)

			for _, subPath := range subPaths {
				fp, err := joinPath(task.siteConfig.baseURL, subPath)
				if err != nil {
					task.fail(fmt.Errorf("resolve card link %q on %s: %w", subPath, resp.Request.URL, err))
					continue
				}
				fullPath := fp.String()

				detailedPageResp, err := task.client.request(task.ctx, requestOptions{
					Method:  http.MethodGet,
					URL:     fullPath,
					Referer: task.siteConfig.cardListURL,
				})
				if err != nil {
					task.fail(fmt.Errorf("fetch card page %s: %w", fullPath, err))
					continue
				}

				doc, err := goquery.NewDocumentFromReader(bytes.NewReader(detailedPageResp.Body))
				if err != nil {
					task.fail(fmt.Errorf("parse card page %s: %w", fullPath, err))
					continue
				}
				log.With("url", fullPath).Debug("Successfully parsed detailed page")
				wgCardSel.Add(1)
				cardSelCh <- cardPage{url: fullPath, task: task, sel: doc.Selection}
			}

			return len(subPaths)
		},
		recentReleaseDistinguisher: "div.p-cards__latest-products ul.c-product__list a",
		recentReleaseExpansionFunc: func(sel *goquery.Selection) *url.Values {
			if hrefAttr, exists := sel.Attr("href"); exists {
				re := regexp.MustCompile(`expansion=(\d+)`)
				if m := re.FindStringSubmatch(hrefAttr); m != nil {
					return &url.Values{
						"view":      {"text"},
						"expansion": {m[1]},
					}
				}
			}
			return nil
		},
		supportTitleNumber: true,
	},
	Japanese: {
		baseURL: "https://ws-tcg.com/",
		baseURLValues: func() url.Values {
			return url.Values{
				"cmd":             {"search"},
				"show_page_count": {"100"},
				"show_small":      {"0"},
			}
		},
		cardListURL:   "https://ws-tcg.com/cardlist/",
		cardSearchURL: "https://ws-tcg.com/cardlist/search",
		languageCode:  language.Japanese,
		// Japanese cards come from the JSON search API (see japanese_api.go),
		// so there are no HTML results-page hooks here.
		recentReleaseDistinguisher: "div.system > ul.expansion-list a[onclick]",
		recentReleaseExpansionFunc: func(sel *goquery.Selection) *url.Values {
			onclickAttr, exists := sel.Attr("onclick")
			if exists {
				// Extract the integer value from the onclick attribute
				parts := strings.Split(onclickAttr, "('")
				if len(parts) >= 2 {
					value := strings.TrimSuffix(parts[1], "')")
					return &url.Values{
						"cmd":             {"search"},
						"show_page_count": {"100"},
						"show_small":      {"0"},
						"parallel":        {"0"},
						"expansion":       {value},
					}
				}
			}
			return nil
		},
		supportTitleNumber: false,
	},
}

// englishCardLinkRE matches the detail-page links on an English search results page.
var englishCardLinkRE = regexp.MustCompile(`href="(/cardlist/\?cardno=[^"]+)"`)

// englishListingCardLinks returns the detail-page paths on an English search
// results page, in page order and without duplicates.
//
// The links are pulled from the raw HTML rather than the parsed DOM on purpose:
// the site occasionally emits an unclosed attribute in a card's rules text
// (e.g. <img src='/.../REST] ...), and the HTML5 parser then swallows the
// following <li> as attribute garbage, so the next card is never seen.
func englishListingCardLinks(body []byte) []string {
	var links []string
	seen := make(map[string]bool)
	for _, m := range englishCardLinkRE.FindAllSubmatch(body, -1) {
		link := html.UnescapeString(string(m[1]))
		if seen[link] {
			continue
		}
		seen[link] = true
		links = append(links, link)
	}
	return links
}

type Booster struct {
	// ReleaseCode is the first set of characters following the / in the card
	// number. See Card.Release for more information.
	ReleaseCode string
	Cards       []Card
}

// cardPage is a fetched card detail page, queued for extraction.
type cardPage struct {
	url  string
	task *scrapeTask
	sel  *goquery.Selection
}

type scrapeTask struct {
	pageURLCh  chan string
	pageRespCh chan *http.Response
	siteConfig siteConfig
	urlValues  url.Values
	// resultCount is the total number of results the site reports for urlValues.
	resultCount int
	lastPage    int
	wgPageScan  *sync.WaitGroup
	client      *Client
	ctx         context.Context

	mu sync.Mutex
	// found is the number of cards discovered across all scanned results pages.
	found int
	// errs collects every page-level failure so the caller can tell a partial
	// result from a complete one.
	errs []error
}

// fail records a page-level failure. It is safe to call from any worker.
func (s *scrapeTask) fail(err error) {
	s.client.log().Error("Page-level scrape failure", "error", err)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, err)
}

// expectedOnPage is how many cards a results page should hold given the
// site's reported result count: a full page everywhere but the last.
func (s *scrapeTask) expectedOnPage(page int) int {
	if page < s.lastPage {
		return s.siteConfig.cardsPerPage
	}
	return s.resultCount - (s.lastPage-1)*s.siteConfig.cardsPerPage
}

// recordPage tallies the cards found on one results page and warns when the
// page came up short, which is what a silently dropped card looks like.
func (s *scrapeTask) recordPage(pageURL *url.URL, found int) {
	s.mu.Lock()
	s.found += found
	s.mu.Unlock()

	page, err := strconv.Atoi(pageURL.Query().Get("page"))
	if err != nil {
		s.client.log().With("url", pageURL).Warn("Couldn't tell which results page this is", "error", err)
		return
	}
	if expected := s.expectedOnPage(page); found != expected {
		s.client.log().With("url", pageURL).Warn("Results page card count mismatch", "expected", expected, "found", found)
	}
}

// fetchResultCount asks the site how many results urlValues matches and
// derives the number of results pages from it.
func (s *scrapeTask) fetchResultCount() error {
	s.client.log().Info(fmt.Sprintf("Getting result count of %q with %v", s.siteConfig.cardSearchURL, s.urlValues))
	count, err := s.client.fetchResultCount(s.ctx, s.siteConfig, s.urlValues)
	if err != nil {
		return err
	}
	s.resultCount = count
	s.lastPage = (count-1)/s.siteConfig.cardsPerPage + 1
	if s.lastPage < 1 {
		s.lastPage = 1
	}
	s.client.log().Info(fmt.Sprintf("%d results over %d pages for %v", count, s.lastPage, s.urlValues))
	return nil
}

// fetchResultCount issues one search request for urlValues and returns the
// result count the site reports for it.
func (c *Client) fetchResultCount(ctx context.Context, siteCfg siteConfig, urlValues url.Values) (int, error) {
	respData, err := c.request(ctx, requestOptions{
		Method:  http.MethodPost,
		URL:     fmt.Sprintf("%v?page=%d", siteCfg.cardSearchURL, 1),
		Referer: siteCfg.cardListURL,
		Form:    urlValues,
	})
	if err != nil {
		return 0, fmt.Errorf("error getting result count: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respData.Body))
	if err != nil {
		return 0, fmt.Errorf("error parsing result count page: %w", err)
	}

	count, err := siteCfg.resultCountFunc(doc)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", respData.Request.URL, err)
	}
	return count, nil
}

// getTasksForRecentReleases returns the search form values for each of the
// recent releases linked from the card list landing page.
func getTasksForRecentReleases(siteCfg siteConfig, doc *goquery.Document) []url.Values {
	var tasks []url.Values
	// Find all <a> elements with onclick attributes within the <ul> element
	doc.Find(siteCfg.recentReleaseDistinguisher).Each(func(i int, sel *goquery.Selection) {
		if v := siteCfg.recentReleaseExpansionFunc(sel); v != nil {
			tasks = append(tasks, *v)
		}
	})
	return tasks
}

func joinPath(baseURL, subPath string) (*url.URL, error) {
	b, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse base URL: %v", err)
	}
	sp, err := url.Parse(subPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse sub path: %v", err)
	}
	return b.ResolveReference(sp), nil
}

func pageFetchWorker(id int, task *scrapeTask) {
	log := task.client.log()
	for link := range task.pageURLCh {
		log.Debug(fmt.Sprintf("ID %d: fetching page %q with params %v", id, link, task.urlValues))
		respData, err := task.client.request(task.ctx, requestOptions{
			Method:  http.MethodPost,
			URL:     link,
			Referer: task.siteConfig.cardListURL,
			Form:    task.urlValues,
		})
		if err != nil {
			task.fail(fmt.Errorf("fetch results page %s: %w", link, err))
			task.wgPageScan.Done()
			continue
		}
		task.pageRespCh <- responseToHTTPResponse(respData)
	}
	log.Info(fmt.Sprintf("Page fetch worker %d done", id))
}

func pageScanWorker(
	id int,
	task *scrapeTask,
	wgCardSel *sync.WaitGroup,
	cardSelCh chan<- cardPage,
) {
	log := task.client.log()
	for resp := range task.pageRespCh {
		log.Debug(fmt.Sprintf("Start scanning page: %v", resp.Request.URL))
		found := task.siteConfig.pageScanParseFunc(task, wgCardSel, cardSelCh, resp)
		resp.Body.Close()
		task.recordPage(resp.Request.URL, found)
		task.wgPageScan.Done()
		log.Debug(fmt.Sprintf("Finish scanning page: %v", resp.Request.URL))
	}
	log.Info(fmt.Sprintf("Page scan worker %d done", id))
}

func getImageWithClient(ctx context.Context, client *Client, url string) (image.Image, error) {
	respData, err := client.request(ctx, requestOptions{
		Method: http.MethodGet,
		URL:    url,
	})
	if err != nil {
		return nil, err
	}
	img, _, decodeErr := image.Decode(bytes.NewReader(respData.Body))
	if decodeErr != nil {
		return nil, decodeErr
	}
	return img, nil
}

func extractWorker(ctx context.Context, client *Client, siteCfg siteConfig, getImages bool, wgCardSel *sync.WaitGroup, cardSelChan <-chan cardPage, cardCh chan<- Card) {
	for page := range cardSelChan {
		c := extractData(siteCfg, page.sel, client.log())
		if c.CardNumber == "" {
			// Either the page wasn't a card page (a block or error page served
			// with 200) or extraction panicked and recovered to a zero Card.
			// A blank card is worse than a missing one, so drop it and make
			// the scrape incomplete.
			page.task.fail(fmt.Errorf("card page %s produced no card", page.url))
			wgCardSel.Done()
			continue
		}
		applyExpansionMetadata(&c, client.resolveExpansionMetadata(ctx, SiteLanguage(siteCfg.languageCode), c))

		if getImages {
			if img, err := getImageWithClient(ctx, client, c.ImageURL); err != nil {
				client.log().Error(fmt.Sprintf("Problem getting image for %s: %v", c.CardNumber, err))
			} else {
				c.Image = img
			}
		}

		cardCh <- c
		wgCardSel.Done()
	}
}

type reducer interface {
	reduce(config reducerConfig)
}

type reducerConfig struct {
	wg     *sync.WaitGroup
	cardCh chan Card
}

type cardListReducer struct {
	cards []Card
}

func (clr *cardListReducer) reduce(rc reducerConfig) {
	for c := range rc.cardCh {
		clr.cards = append(clr.cards, c)
	}
	rc.wg.Done()
}

type boosterReducer struct {
	boosterMap map[string]Booster
}

func (br *boosterReducer) reduce(rc reducerConfig) {
	for c := range rc.cardCh {
		boosterCode := c.Release
		boosterObj := br.boosterMap[boosterCode]
		boosterObj.ReleaseCode = boosterCode

		boosterObj.Cards = append(boosterObj.Cards, c)
		br.boosterMap[boosterCode] = boosterObj
	}
	rc.wg.Done()
}

type Config struct {
	// The website's internal code for each expansion. The value is language-specific.
	// For example,
	//   159 is "BanG Dream! Girls Band Party Premium Booster" in EN
	//   159 is "Monogatari Series: Second Season"
	ExpansionNumber int
	GetAllRarities  bool
	GetImages       bool
	GetRecent       bool
	Language        SiteLanguage
	PageStart       int
	Reverse         bool
	SetCode         []string
	// The website's internal code for each set. The value is language-specific.
	// For example
	//   159 is "Tokyo Revengers" in EN
	//   159 isn't supported in JP
	TitleNumber int
}

// searchValues translates cfg into the site config and search form values
// that select the cards it describes. It is the single source of truth for
// that translation so a count and a scrape of the same Config agree.
func searchValues(cfg Config) (siteConfig, url.Values, error) {
	siteCfg, ok := siteConfigs[cfg.Language]
	if !ok {
		return siteConfig{}, nil, fmt.Errorf("unsupported language: %v", cfg.Language)
	}

	urlValues := siteCfg.baseURLValues()
	if cfg.ExpansionNumber != 0 {
		switch cfg.Language {
		case English:
			// "expansion" also works, but the website uses "expansion_name", so use "expansion" to
			// stay in line with the website
			urlValues.Add("expansion_name", strconv.Itoa(cfg.ExpansionNumber))
		case Japanese:
			urlValues.Add("expansion", strconv.Itoa(cfg.ExpansionNumber))
		}
	}
	if cfg.TitleNumber != 0 {
		if !siteCfg.supportTitleNumber {
			return siteConfig{}, nil, fmt.Errorf("can't use title number on %v site", cfg.Language)
		}
		urlValues.Add("title", strconv.Itoa(cfg.TitleNumber))
	}
	if cfg.GetAllRarities {
		urlValues.Add("parallel", "0")
	} else {
		urlValues.Add("parallel", "1")
	}
	if len(cfg.SetCode) > 0 {
		switch cfg.Language {
		case English:
			urlValues.Add("keyword_or", strings.Join(cfg.SetCode, " "))
			urlValues.Add("keyword_type[]", "no")
		case Japanese:
			urlValues.Add("title_number", fmt.Sprintf("##%s##", strings.Join(cfg.SetCode, "##")))
		}
	}
	return siteCfg, urlValues, nil
}

// CardCount returns the number of cards the site lists for cfg, honouring
// Language, ExpansionNumber, TitleNumber, SetCode and GetAllRarities exactly
// as CardsStream does, so a count and a scrape of the same Config agree.
// It issues one request.
//
// GetRecent is rejected because it describes several searches, not one.
// PageStart, Reverse and GetImages only affect how a scrape proceeds and are
// ignored.
//
// Like every other request, the count goes through the client's response
// cache if one was configured with WithCache, so a cached client reports
// the cached count until the TTL expires. A consumer polling for changes
// should use a client without a cache.
func (c *Client) CardCount(ctx context.Context, cfg Config) (int, error) {
	if cfg.GetRecent {
		return 0, fmt.Errorf("can't count cards with GetRecent: it selects several searches")
	}
	siteCfg, urlValues, err := searchValues(cfg)
	if err != nil {
		return 0, err
	}

	if cfg.Language == Japanese {
		page, err := c.fetchJapaneseCardSearch(ctx, urlValues, 1)
		if err != nil {
			return 0, err
		}
		return page.Total, nil
	}
	return c.fetchResultCount(ctx, siteCfg, urlValues)
}

// CardsStream sends every card matching cfg to cardCh and closes cardCh when
// it returns, whether or not it succeeded.
//
// If some results pages or card pages could not be fetched, the cards that
// were fetched are still sent and the returned error wraps ErrIncomplete, so
// callers can tell a partial result from a complete one.
func (c *Client) CardsStream(ctx context.Context, cfg Config, cardCh chan<- Card) error {
	// Deferring the close is only safe because nothing returns after the
	// extract workers (which send on cardCh) are started; every early return
	// below happens before any worker exists. Keep it that way.
	defer close(cardCh)

	siteCfg, urlValues, err := searchValues(cfg)
	if err != nil {
		return err
	}
	c.log().Info(fmt.Sprintf("Fetching %v cards", cfg.Language))
	c.log().Info("Streaming cards", "config", cfg)

	if cfg.Language == Japanese {
		return c.cardsStreamJapanese(ctx, cfg, urlValues, cardCh)
	}

	newTask := func(values url.Values) *scrapeTask {
		return &scrapeTask{
			siteConfig: siteCfg,
			urlValues:  values,
			client:     c,
			ctx:        ctx,
		}
	}
	var scrapeTasks []*scrapeTask
	if cfg.GetRecent {
		resp, err := c.request(ctx, requestOptions{
			Method: http.MethodGet,
			URL:    siteCfg.cardListURL,
		})
		if err != nil {
			return fmt.Errorf("error getting recent: %v", err)
		}
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
		if err != nil {
			return fmt.Errorf("error parsing recent: %v", err)
		}
		recentTasks := getTasksForRecentReleases(siteCfg, doc)
		if len(recentTasks) == 0 {
			return fmt.Errorf("no recent releases found on %s", siteCfg.cardListURL)
		}
		for _, recent := range recentTasks {
			c.log().Debug(fmt.Sprintf("recent scrape task=%v", recent))
			scrapeTasks = append(scrapeTasks, newTask(recent))
		}
	} else {
		scrapeTasks = append(scrapeTasks, newTask(urlValues))
	}

	loopNum := 0
	for _, st := range scrapeTasks {
		if err := st.fetchResultCount(); err != nil {
			return err
		}
		loopNum += st.lastPage
		st.pageURLCh = make(chan string, st.lastPage)
		st.pageRespCh = make(chan *http.Response, maxScrapeWorker)
		st.wgPageScan = &sync.WaitGroup{}
		st.wgPageScan.Add(st.lastPage)
	}

	c.log().Debug(fmt.Sprintf("Number of loop %v", loopNum))

	var wgScanner, wgCardSel sync.WaitGroup
	cardSelCh := make(chan cardPage, maxLocalWorker)
	for i := 0; i < maxLocalWorker; i++ {
		go extractWorker(ctx, c, siteCfg, cfg.GetImages, &wgCardSel, cardSelCh, cardCh)
	}
	for _, st := range scrapeTasks {
		wgScanner.Add(1)
		go func(s *scrapeTask) {
			// Wait for page scanning to finish instead of the fetch workers because
			// sometimes the scanners put work back in the fetch channel.
			s.wgPageScan.Wait()
			close(s.pageURLCh)
			close(s.pageRespCh)
			wgScanner.Done()
		}(st)
		for i := 0; i < maxScrapeWorker; i++ {
			go pageFetchWorker(i, st)
			go pageScanWorker(i, st, &wgCardSel, cardSelCh)
		}
		for i := 1; i <= st.lastPage; i++ {
			if i < cfg.PageStart {
				// Skip everything before this page. Mark as done so the routines aren't waiting for it.
				st.wgPageScan.Done()
				continue
			}

			id := i
			if cfg.Reverse {
				id = st.lastPage - i + 1
			}
			st.pageURLCh <- fmt.Sprintf("%v?page=%d", siteCfg.cardSearchURL, id)
		}
	}

	wgScanner.Wait()
	wgCardSel.Wait()
	close(cardSelCh)

	var errs []error
	for _, st := range scrapeTasks {
		// A partial scrape (PageStart) can't be expected to match the site's count.
		if cfg.PageStart <= 1 && st.found != st.resultCount {
			st.fail(resultCountMismatch(st.resultCount, st.found, st.urlValues))
		}
		errs = append(errs, st.errs...)
	}
	return incompleteError(errs)
}

// resultCountMismatch describes a scrape that found a different number of
// cards than the site's result count said it would.
func resultCountMismatch(expected, found int, params url.Values) error {
	return fmt.Errorf("site reports %d results but found %d for %v", expected, found, params)
}

// incompleteError wraps the collected page-level failures of a scrape in
// ErrIncomplete, or returns nil if there were none.
func incompleteError(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrIncomplete, errors.Join(errs...))
}

func (c *Client) aggregate(ctx context.Context, cfg Config, r reducer) error {
	cardCh := make(chan Card, maxScrapeWorker)

	var wg sync.WaitGroup
	wg.Add(1)

	reducerCfg := reducerConfig{
		wg:     &wg,
		cardCh: cardCh,
	}

	go r.reduce(reducerCfg)

	err := c.CardsStream(ctx, cfg, cardCh)

	wg.Wait()

	return err
}

func (c *Client) Cards(ctx context.Context, cfg Config) ([]Card, error) {
	var reducer cardListReducer
	err := c.aggregate(ctx, cfg, &reducer)

	return reducer.cards, err
}

func (c *Client) Boosters(ctx context.Context, cfg Config) (map[string]Booster, error) {
	reducer := boosterReducer{boosterMap: make(map[string]Booster)}
	err := c.aggregate(ctx, cfg, &reducer)

	return reducer.boosterMap, err
}

// ExpansionList returns a map of expansion numbers to their titles for the
// specified language in the Config.
func (c *Client) ExpansionList(ctx context.Context, cfg Config) (map[int]string, error) {
	var siteCfg siteConfig
	if sc, ok := siteConfigs[cfg.Language]; !ok {
		return nil, fmt.Errorf("unsupported language: %v", cfg.Language)
	} else {
		siteCfg = sc
		c.log().Info(fmt.Sprintf("Fetching %v expansion list", cfg.Language))
	}

	if cfg.Language == Japanese {
		options, err := c.japaneseFilterOptions(ctx)
		if err != nil {
			return nil, err
		}
		eMap := make(map[int]string, len(options.Expansions))
		for _, exp := range options.Expansions {
			if exp.ID != 0 && exp.Name != "" {
				eMap[exp.ID] = exp.Name
			}
		}
		return eMap, nil
	}

	respData, err := c.request(ctx, requestOptions{
		Method:  http.MethodPost,
		URL:     siteCfg.cardListURL,
		Referer: siteCfg.cardListURL,
		Form:    url.Values{},
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't read page: %v", err)
	}

	resp := responseToHTTPResponse(respData)
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respData.Body))
	if err != nil {
		return nil, fmt.Errorf("goquery error for page %q: %v", resp.Request.URL, err)
	}

	expansionList := doc.Find("select#expansion option")
	if expansionList.Length() == 0 && resp.StatusCode == http.StatusOK {
		return nil, fmt.Errorf("couldn't find expansion list")
	}

	eMap := make(map[int]string)
	expansionList.Each(func(i int, s *goquery.Selection) {
		val, exists := s.Attr("value")
		val = strings.TrimSpace(val)
		if !exists || val == "" {
			// This is probably the "All" option
			c.log().Warn(fmt.Sprintf("Option %q had no value", s.Text()))
			return
		}
		if v, err := strconv.Atoi(val); err != nil {
			c.log().Error(fmt.Sprintf("Error parsing expansion value: %v", err))
		} else {
			eMap[v] = s.Text()
		}

	})

	return eMap, nil
}

func getWithHeaders(client *http.Client, endpoint string, referer string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	setDefaultHeaders(req, referer)

	return client.Do(req)
}

func postFormWithHeaders(client *http.Client, endpoint string, form url.Values, referer string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	setDefaultHeaders(req, referer)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return client.Do(req)
}

func setDefaultHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}
