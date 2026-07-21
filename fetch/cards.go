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
	"fmt"
	"image"
	"log/slog"
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

type siteConfig struct {
	baseURL                    string
	baseURLValues              func() url.Values
	cardListURL                string
	cardSearchURL              string
	languageCode               language.Tag
	lastPageFunc               func(doc *goquery.Document, logger *slog.Logger) int
	pageScanParseFunc          func(task *scrapeTask, wgCardSel *sync.WaitGroup, cardSelCh chan<- *goquery.Selection, resp *http.Response) (pageDone bool)
	recentReleaseDistinguisher string
	recentRelaseExpansionFunc  func(page *goquery.Selection) *url.Values
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
		languageCode:  language.English,
		lastPageFunc: func(doc *goquery.Document, logger *slog.Logger) int {
			numCardsS := doc.Find(".c-search__results-item span").First().Text()
			numCardsS = strings.TrimSpace(numCardsS)
			numCardsS = strings.ReplaceAll(numCardsS, ",", "")
			numCards, err := strconv.Atoi(numCardsS)
			if err != nil {
				loggerOrDefault(logger).Error(fmt.Sprintf("Couldn't get num cards: %v", err))
				return 1
			}
			// As of 2024-9-3, there are 15 cards per "page".
			// TODO: figure out a better way to get the total number of pages
			return (numCards-1)/15 + 1
		},
		pageScanParseFunc: func(task *scrapeTask, wgCardSel *sync.WaitGroup, cardSelCh chan<- *goquery.Selection, resp *http.Response) (pageDone bool) {
			log := task.client.log()
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.With("url", resp.Request.URL).Error(fmt.Sprintf("Couldn't parse result page: %v", err))
				return false
			}
			resultList := doc.Find(".p_cards__results-box ul li")

			if resultList.Length() == 0 && resp.StatusCode == http.StatusOK {
				log.With("url", resp.Request.URL).Warn("No cards on response page")
			} else {
				log.With("url", resp.Request.URL).Debug("Found cards!")
				resultList.Each(func(i int, s *goquery.Selection) {
					subPath, exists := s.Find("a").First().Attr("href")
					if !exists {
						log.With("url", resp.Request.URL).Error(fmt.Sprintf("Error getting sub path: %v", err))
						return
					}
					fp, err := joinPath(task.siteConfig.baseURL, subPath)
					if err != nil {
						log.With("url", resp.Request.URL).Error(fmt.Sprintf("Error getting full path: %v", err))
						return
					}
					fullPath := fp.String()

					detailedPageResp, err := task.client.request(task.ctx, requestOptions{
						Method:  http.MethodGet,
						URL:     fullPath,
						Referer: task.siteConfig.cardListURL,
					})
					if err != nil {
						log.With("url", fullPath).Error("Failed to get detailed page", "error", err)
						return
					}

					doc, err := goquery.NewDocumentFromReader(bytes.NewReader(detailedPageResp.Body))
					if err != nil {
						log.With("url", fullPath).Error(fmt.Sprintf("Couldn't parse detailed page: %v", err))
						return
					}
					log.With("url", fullPath).Debug("Successfully parsed detailed page")
					cardDetails := doc.Selection
					wgCardSel.Add(1)
					cardSelCh <- cardDetails
				})
			}

			return true
		},
		recentReleaseDistinguisher: "div.p-cards__latest-products ul.c-product__list a",
		recentRelaseExpansionFunc: func(sel *goquery.Selection) *url.Values {
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
		lastPageFunc: func(doc *goquery.Document, logger *slog.Logger) int {
			all := doc.Find(".pager .next")

			last, _ := strconv.Atoi(all.Prev().First().Text())
			// default is 1, there no .pager .next if it's the only page
			if last == 0 {
				last = 1
			}
			return last
		},
		pageScanParseFunc: func(task *scrapeTask, wgCardSel *sync.WaitGroup, cardSelCh chan<- *goquery.Selection, resp *http.Response) (pageDone bool) {
			log := task.client.log()
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				task.pageURLCh <- resp.Request.URL.String()
				log.With("url", resp.Request.URL).Error(fmt.Sprintf("Couldn't parse result page: %v", err))
				return false
			}
			resultTable := doc.Find(".search-result-table tr")

			if resultTable.Length() == 0 && resp.StatusCode == http.StatusOK {
				log.With("url", resp.Request.URL).Warn("No cards on response page")
			} else {
				log.With("url", resp.Request.URL).Debug("Found cards!")
				resultTable.Each(func(i int, s *goquery.Selection) {
					wgCardSel.Add(1)
					cardSelCh <- s
				})
			}

			return true
		},
		recentReleaseDistinguisher: "div.system > ul.expansion-list a[onclick]",
		recentRelaseExpansionFunc: func(sel *goquery.Selection) *url.Values {
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

type Booster struct {
	// ReleaseCode is the first set of characters following the / in the card
	// number. See Card.Release for more information.
	ReleaseCode string
	Cards       []Card
}

type scrapeTask struct {
	pageURLCh  chan string
	pageRespCh chan *http.Response
	siteConfig siteConfig
	urlValues  url.Values
	lastPage   int
	wgPageScan *sync.WaitGroup
	client     *Client
	ctx        context.Context
}

func (s *scrapeTask) getLastPage() (int, error) {
	s.client.log().Info(fmt.Sprintf("Getting last page of %q with %v", s.siteConfig.cardSearchURL, s.urlValues))
	respData, err := s.client.request(s.ctx, requestOptions{
		Method:  http.MethodPost,
		URL:     fmt.Sprintf("%v?page=%d", s.siteConfig.cardSearchURL, 1),
		Referer: s.siteConfig.cardListURL,
		Form:    s.urlValues,
	})
	if err != nil {
		return 0, fmt.Errorf("error getting last page: %v", err)
	}
	resp := responseToHTTPResponse(respData)
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respData.Body))
	if err != nil {
		return 0, fmt.Errorf("error parsing last page: %v", err)
	}

	last := s.siteConfig.lastPageFunc(doc, s.client.log())

	s.client.log().With("url", resp.Request.URL).Info(fmt.Sprintf("Last page is %d for %v", last, s.urlValues))
	s.lastPage = last
	return last, nil
}

func getTasksForRecentReleases(siteCfg siteConfig, doc *goquery.Document) []scrapeTask {
	var tasks []scrapeTask
	// Find all <a> elements with onclick attributes within the <ul> element
	doc.Find(siteCfg.recentReleaseDistinguisher).Each(func(i int, sel *goquery.Selection) {
		if v := siteCfg.recentRelaseExpansionFunc(sel); v != nil {

			tasks = append(tasks, scrapeTask{urlValues: *v})
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
			log.With("url", link).Error("Failed page fetch", "error", err)
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
	cardSelCh chan<- *goquery.Selection,
) {
	log := task.client.log()
	for resp := range task.pageRespCh {
		log.Debug(fmt.Sprintf("Start scanning page: %v", resp.Request.URL))
		if task.siteConfig.pageScanParseFunc(task, wgCardSel, cardSelCh, resp) {
			task.wgPageScan.Done()
		}
		resp.Body.Close()
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

func extractWorker(ctx context.Context, client *Client, siteCfg siteConfig, getImages bool, wgCardSel *sync.WaitGroup, cardSelChan <-chan *goquery.Selection, cardCh chan<- Card) {
	for s := range cardSelChan {
		c := extractData(siteCfg, s, client.log())
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

func (c *Client) CardsStream(ctx context.Context, cfg Config, cardCh chan<- Card) error {
	var siteCfg siteConfig
	if sc, ok := siteConfigs[cfg.Language]; !ok {
		return fmt.Errorf("unsupported language: %v", cfg.Language)
	} else {
		siteCfg = sc
		c.log().Info(fmt.Sprintf("Fetching %v cards", cfg.Language))
	}

	c.log().Info("Streaming cards", "config", cfg)

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
			return fmt.Errorf("can't use title number on %v site", cfg.Language)
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

	if cfg.Language == Japanese {
		return c.cardsStreamJapanese(ctx, cfg, urlValues, cardCh)
	}

	var scrapeTasks []*scrapeTask
	defaultScrapeTask := scrapeTask{
		siteConfig: siteCfg,
		urlValues:  urlValues,
		client:     c,
		ctx:        ctx,
	}
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
		for _, recent := range getTasksForRecentReleases(siteCfg, doc) {
			copyTask := defaultScrapeTask
			copyTask.urlValues = recent.urlValues
			c.log().Debug(fmt.Sprintf("default scrape task=%v, recent=%v", defaultScrapeTask, recent))
			scrapeTasks = append(scrapeTasks, &copyTask)
		}
	} else {
		scrapeTasks = append(scrapeTasks, &defaultScrapeTask)
	}

	loopNum := 0
	for _, st := range scrapeTasks {
		lastPage, err := st.getLastPage()
		if err != nil {
			return err
		}
		loopNum += lastPage
		st.pageURLCh = make(chan string, lastPage)
		st.pageRespCh = make(chan *http.Response, maxScrapeWorker)
		st.wgPageScan = &sync.WaitGroup{}
		st.wgPageScan.Add(lastPage)
	}

	c.log().Debug(fmt.Sprintf("Number of loop %v", loopNum))

	var wgScanner, wgCardSel sync.WaitGroup
	cardSelCh := make(chan *goquery.Selection, maxLocalWorker)
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
	close(cardCh)

	return nil
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
	var reducer boosterReducer
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
