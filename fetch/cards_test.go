package fetch

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

// fastTestOptions builds a client that talks to a stub transport as quickly
// as the client allows: no robots lookup, no retries, an effectively
// unlimited rate limiter, and enough network slots for the scrape workers
// to overlap. The per-request jitter in Client.request is not configurable,
// so each request still costs up to 150ms; overlapping them is what keeps
// the multi-request tests fast.
func fastTestOptions() []Option {
	return []Option{
		WithRespectRobots(false),
		WithMaxRetries(0),
		WithRequestsPerSecond(1000),
		WithBurst(100),
		WithNetworkConcurrency(10),
	}
}

func TestRecentSwitch_en(t *testing.T) {
	expectedExpansion := []string{
		"228",
		"227",
		"226",
		"225",
	}
	f, err := os.Open("mockws-en/recent.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}
	recentTasks := getTasksForRecentReleases(siteConfigs[English], doc)
	if len(recentTasks) != len(expectedExpansion) {
		t.Errorf("Didn't get enough tasks. Got %d, want %d", len(recentTasks), len(expectedExpansion))
	}

	for _, task := range recentTasks {
		expansion := task.Get("expansion")
		if !slices.Contains(expectedExpansion, expansion) {
			t.Errorf("Did not expect %q expansion", expansion)
		}
	}
}

func TestCardsJapaneseUsesSearchJSONPagination(t *testing.T) {
	client, err := NewClient(fastTestOptions()...)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	var searchPages []string
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/manage/CardListUser/filter-options":
			return newHTTPResponse(r, http.StatusOK, nil, `{
				"expansions": [
					{"id":1,"name":"D.C. D.C.II","category":"1","disp_flg":1,"newest_flg":0,"create_date":"2026-01-01T00:00:00+00:00"}
				]
			}`), nil
		case "/manage/CardListUser/searchJson":
			searchPages = append(searchPages, r.URL.Query().Get("page"))
			if got := r.URL.Query().Get("expansion"); got != "1" {
				t.Fatalf("unexpected expansion query: %q", got)
			}
			if r.URL.Query().Get("page") == "2" {
				return newHTTPResponse(r, http.StatusOK, nil, `{
					"items": [
						{"id":2,"card_number":"DC/W01-002","card_name":"朝倉 音姫","card_kind":"2","color":"[[yellow.gif]]","level":"1","cost":"1","power":"6000","soul":"[[soul.gif]]","card_trigger":"[[soul.gif]]","text":"【自】 アンコール","picture":"d/dc_w01/dc_w01_002.png","expansion":1,"rare":"RR","feature1":"魔法","side":"-1"}
					],
					"total": 2,
					"page": 2,
					"limit": 1,
					"page_count": 2
				}`), nil
			}
			return newHTTPResponse(r, http.StatusOK, nil, `{
				"items": [
					{"id":1,"card_number":"DC/W01-001","card_name":"学園長のさくら","card_kind":"2","color":"[[yellow.gif]]","level":"0","cost":"0","power":"500","soul":"[[soul.gif]]","card_trigger":"-","text":"【自】 絆","picture":"d/dc_w01/dc_w01_001.png","expansion":1,"rare":"RR","feature1":"魔法","side":"-1"}
				],
				"total": 2,
				"page": 1,
				"limit": 1,
				"page_count": 2
			}`), nil
		case "/wp-json/wp/v2/products":
			if got := r.URL.Query().Get("search"); got != "D.C. D.C.II" {
				t.Fatalf("unexpected product search: %q", got)
			}
			return newHTTPResponse(r, http.StatusOK, nil, `[{
				"slug": "bp-dc",
				"link": "https://ws-tcg.com/products/bp-dc/",
				"title": {"rendered": "D.C. D.C.II"},
				"products_cat": [15]
			}]`), nil
		case "/products/bp-dc/":
			return newHTTPResponse(r, http.StatusOK, nil, `
<div class="products__specs">
  <h1 class="products__articleName">ブースターパック D.C. D.C.II</h1>
</div>`), nil
		default:
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		return nil, nil
	})

	cards, err := client.Cards(context.Background(), Config{
		Language:        Japanese,
		ExpansionNumber: 1,
	})
	if err != nil {
		t.Fatalf("Cards failed: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	if !slices.Equal(searchPages, []string{"1", "2"}) {
		t.Fatalf("unexpected pages: %v", searchPages)
	}
	if cards[0].ExpansionName != "D.C. D.C.II" || cards[1].ExpansionName != "D.C. D.C.II" {
		t.Fatalf("unexpected expansion names: %#v", cards)
	}
	for _, card := range cards {
		if card.ExpansionSlug != "bp-dc" {
			t.Fatalf("unexpected ExpansionSlug: %q", card.ExpansionSlug)
		}
		if card.ExpansionProductDisplayName != "ブースターパック D.C. D.C.II" {
			t.Fatalf("unexpected ExpansionProductDisplayName: %q", card.ExpansionProductDisplayName)
		}
		if card.ExpansionProductURL != "https://ws-tcg.com/products/bp-dc/" {
			t.Fatalf("unexpected ExpansionProductURL: %q", card.ExpansionProductURL)
		}
		if card.ExpansionSourceType != ExpansionSourceTypeProductPage {
			t.Fatalf("unexpected ExpansionSourceType: %q", card.ExpansionSourceType)
		}
	}
}

func TestRecentJapaneseExpansionValuesUsesFilterOptions(t *testing.T) {
	options := japaneseFilterOptions{Expansions: []japaneseExpansion{
		{ID: 1, Name: "old", DispFlg: 1, NewestFlg: 0, CreateDate: "2026-01-01T00:00:00+00:00"},
		{ID: 2, Name: "latest", DispFlg: 1, NewestFlg: 1, CreateDate: "2026-03-01T00:00:00+00:00"},
		{ID: 3, Name: "hidden", DispFlg: 0, NewestFlg: 1, CreateDate: "2026-04-01T00:00:00+00:00"},
		{ID: 4, Name: "newer", DispFlg: 1, NewestFlg: 1, CreateDate: "2026-04-01T00:00:00+00:00"},
	}}

	got := recentJapaneseExpansionValues(options)
	if len(got) != 2 {
		t.Fatalf("expected 2 recent expansions, got %d", len(got))
	}
	if got[0].Get("expansion") != "4" || got[1].Get("expansion") != "2" {
		t.Fatalf("unexpected expansion order: %v", got)
	}
}

func TestEnglishListingCardLinks_unclosedImgMidPage(t *testing.T) {
	body, err := os.ReadFile("mockws-en/listing_unclosed_img_mid.html")
	if err != nil {
		t.Fatal(err)
	}

	// Keep the fixture honest: the DOM path must actually lose the card that
	// follows the unclosed attribute, otherwise this test proves nothing.
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Find(".p_cards__results-box ul li").Length(); got != 2 {
		t.Fatalf("expected goquery to find 2 <li> for 3 cards, got %d", got)
	}

	want := []string{
		"/cardlist/?cardno=BM/S15-E102&view=text",
		"/cardlist/?cardno=BM/S15-E103&view=text",
		"/cardlist/?cardno=BM/S15-E104&view=text",
	}
	if got := englishListingCardLinks(body); !slices.Equal(got, want) {
		t.Fatalf("englishListingCardLinks = %v, want %v", got, want)
	}
}

func TestEnglishListingCardLinks_unclosedImgLastOnPage(t *testing.T) {
	body, err := os.ReadFile("mockws-en/listing_unclosed_img_last.html")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"/cardlist/?cardno=BM/S15-E102&view=text",
		"/cardlist/?cardno=BM/S15-E103&view=text",
	}
	if got := englishListingCardLinks(body); !slices.Equal(got, want) {
		t.Fatalf("englishListingCardLinks = %v, want %v", got, want)
	}
}

func TestEnglishListingCardLinks_dedupesAndUnescapes(t *testing.T) {
	body := []byte(`
<a href="/cardlist/?cardno=BM/S15-E101&amp;view=text">a</a>
<a href="/cardlist/?cardno=BM/S15-E101&view=text">b</a>
<a href="/cardlist/?cardno=BM/S15-E102&view=text">c</a>
<a href="/cardlist/?page=2">not a card</a>`)

	want := []string{
		"/cardlist/?cardno=BM/S15-E101&view=text",
		"/cardlist/?cardno=BM/S15-E102&view=text",
	}
	if got := englishListingCardLinks(body); !slices.Equal(got, want) {
		t.Fatalf("englishListingCardLinks = %v, want %v", got, want)
	}
}

func englishDetailPage(cardNumber, name string) string {
	return englishDetailPageWithText(cardNumber, name, "【CONT】 Nothing.")
}

// englishDetailPageWithText builds a minimal EN card detail page whose rules
// text is the given HTML fragment.
func englishDetailPageWithText(cardNumber, name, textHTML string) string {
	return `
<div class="p-cards__detail-wrapper">
	<div class="image"><img src="/wordpress/wp-content/images/cardimages/b/bm_s15/x.png" alt="" decoding="async"></div>
	<div class="p-cards__detail-textarea">
	<p class="number">` + cardNumber + `</p>
	<p class="ttl u-mt-14 u-mt-16-sp">` + name + `</p>
	<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
		<dl><dt>Expansion</dt><dd>BAKEMONOGATARI</dd></dl>
		<dl><dt>Card Type</dt><dd>Character</dd></dl>
		<dl><dt>Rarity</dt><dd>TD</dd></dl>
		<dl><dt>Side</dt><dd><img src="/cardlist/partimages/s.gif" alt="" decoding="async"></dd></dl>
		<dl><dt>Color</dt><dd><img src="/wordpress/wp-content/images/partimages/red.gif" alt="" decoding="async"></dd></dl>
	</div>
	<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
		<dl><dt>Level</dt><dd>0</dd></dl>
		<dl><dt>Cost</dt><dd>0</dd></dl>
		<dl><dt>Power</dt><dd>500</dd></dl>
		<dl><dt>Soul</dt><dd><img src="/wordpress/wp-content/images/partimages/soul.gif" alt="" decoding="async"></dd></dl>
		<dl><dt>Trigger</dt><dd>-</dd></dl>
	</div>
	<div class="p-cards__detail u-mt-22 u-mt-40-sp"><p>` + textHTML + `</p></div>
	<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp"><p>-</p></div>
	<p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©SOMEONE</p>
	</div>
</div>
<footer><div class="copyrights"><p>©VisualArt's ©Bushiroad</p></div></footer>`
}

func TestCardsEnglishSurvivesUnclosedImgOnListingPage(t *testing.T) {
	listing, err := os.ReadFile("mockws-en/listing_unclosed_img_mid.html")
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(fastTestOptions()...)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	var detailRequests []string
	var mu sync.Mutex
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cardlist/searchresults/":
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Errorf("unexpected results page: %q", got)
			}
			return newHTTPResponse(r, http.StatusOK, nil, string(listing)), nil
		case "/cardlist/":
			cardNo := r.URL.Query().Get("cardno")
			if cardNo == "" {
				t.Errorf("unexpected cardlist request: %s", r.URL.String())
			}
			mu.Lock()
			detailRequests = append(detailRequests, cardNo)
			mu.Unlock()
			return newHTTPResponse(r, http.StatusOK, nil, englishDetailPage(cardNo, "Card "+cardNo)), nil
		case "/prcards/":
			return newHTTPResponse(r, http.StatusOK, nil, "<html></html>"), nil
		default:
			// Don't t.Fatal here: this runs on a worker goroutine, and killing
			// it would hang the scrape instead of failing the test.
			t.Errorf("unexpected URL: %s", r.URL.String())
			return newHTTPResponse(r, http.StatusNotFound, nil, ""), nil
		}
	})

	cards, err := client.Cards(context.Background(), Config{
		Language:        English,
		ExpansionNumber: 12,
		GetAllRarities:  true,
	})
	if err != nil {
		t.Fatalf("Cards failed: %v", err)
	}

	var got []string
	for _, c := range cards {
		got = append(got, c.CardNumber)
	}
	slices.Sort(got)
	want := []string{"BM/S15-E102", "BM/S15-E103", "BM/S15-E104"}
	if !slices.Equal(got, want) {
		t.Fatalf("card numbers = %v, want %v", got, want)
	}
	slices.Sort(detailRequests)
	if !slices.Equal(detailRequests, want) {
		t.Fatalf("detail page requests = %v, want %v", detailRequests, want)
	}
}

// englishListingPage builds a well-formed EN search results page reporting
// count total results and linking to the given cards.
func englishListingPage(count int, cardNos ...string) string {
	var b strings.Builder
	b.WriteString(`<html><body><div class="p_cards__results">
<p class="c-search__results-item">Search Results<span>` + strconv.Itoa(count) + `</span>items.</p>
<div class="p_cards__results-box"><ul>`)
	for _, no := range cardNos {
		b.WriteString(`<li><a href="/cardlist/?cardno=` + no + `&view=text"><p class="number">` + no + `</p></a></li>`)
	}
	b.WriteString(`</ul></div></div></body></html>`)
	return b.String()
}

// syncBuffer is a bytes.Buffer that is safe to write from scrape workers.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newEnglishStubClient returns a fast client whose transport serves listing
// pages by page number and detail pages via detail. Unknown URLs get a 404
// and fail the test without killing the worker goroutine that asked.
func newEnglishStubClient(t *testing.T, logs *syncBuffer, listings map[string]string, detail func(cardNo string) (int, string)) *Client {
	t.Helper()
	opts := fastTestOptions()
	if logs != nil {
		opts = append(opts, WithLogger(slog.New(slog.NewTextHandler(logs, nil))))
	}
	client, err := NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(client.Close)

	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/cardlist/searchresults/":
			if body, ok := listings[r.URL.Query().Get("page")]; ok {
				return newHTTPResponse(r, http.StatusOK, nil, body), nil
			}
			return newHTTPResponse(r, http.StatusInternalServerError, nil, ""), nil
		case "/cardlist/":
			status, body := detail(r.URL.Query().Get("cardno"))
			return newHTTPResponse(r, status, nil, body), nil
		case "/prcards/":
			return newHTTPResponse(r, http.StatusOK, nil, "<html></html>"), nil
		default:
			t.Errorf("unexpected URL: %s", r.URL.String())
			return newHTTPResponse(r, http.StatusNotFound, nil, ""), nil
		}
	})
	return client
}

func okDetailPage(cardNo string) (int, string) {
	return http.StatusOK, englishDetailPage(cardNo, "Card "+cardNo)
}

func cardNumbers(cards []Card) []string {
	var nums []string
	for _, c := range cards {
		nums = append(nums, c.CardNumber)
	}
	slices.Sort(nums)
	return nums
}

func TestScrapeTaskExpectedOnPage(t *testing.T) {
	tests := []struct {
		count    int
		lastPage int
		perPage  []int
	}{
		{count: 19, lastPage: 2, perPage: []int{15, 4}},
		{count: 15, lastPage: 1, perPage: []int{15}},
		{count: 30, lastPage: 2, perPage: []int{15, 15}},
		{count: 0, lastPage: 1, perPage: []int{0}},
	}
	for _, tt := range tests {
		task := &scrapeTask{siteConfig: siteConfigs[English], resultCount: tt.count}
		task.lastPage = (tt.count-1)/task.siteConfig.cardsPerPage + 1
		if task.lastPage < 1 {
			task.lastPage = 1
		}
		if task.lastPage != tt.lastPage {
			t.Errorf("count %d: lastPage = %d, want %d", tt.count, task.lastPage, tt.lastPage)
			continue
		}
		for i, want := range tt.perPage {
			if got := task.expectedOnPage(i + 1); got != want {
				t.Errorf("count %d page %d: expectedOnPage = %d, want %d", tt.count, i+1, got, want)
			}
		}
	}
}

func TestCardsEnglishFailedResultsPageIsAnError(t *testing.T) {
	// 16 results = 2 pages; only page 1 is served.
	listings := map[string]string{
		"1": englishListingPage(16, "AB/W31-E001", "AB/W31-E002"),
	}
	client := newEnglishStubClient(t, nil, listings, okDetailPage)

	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if err == nil || !strings.Contains(err.Error(), "page=2") {
		t.Fatalf("error should name the failed page, got: %v", err)
	}
	want := []string{"AB/W31-E001", "AB/W31-E002"}
	if got := cardNumbers(cards); !slices.Equal(got, want) {
		t.Fatalf("partial cards = %v, want %v", got, want)
	}
}

func TestCardsEnglishFailedDetailPageIsAnError(t *testing.T) {
	listings := map[string]string{
		"1": englishListingPage(3, "AB/W31-E001", "AB/W31-E002", "AB/W31-E003"),
	}
	client := newEnglishStubClient(t, nil, listings, func(cardNo string) (int, string) {
		if cardNo == "AB/W31-E002" {
			return http.StatusInternalServerError, ""
		}
		return okDetailPage(cardNo)
	})

	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if !strings.Contains(err.Error(), "AB/W31-E002") {
		t.Fatalf("error should name the failed card, got: %v", err)
	}
	want := []string{"AB/W31-E001", "AB/W31-E003"}
	if got := cardNumbers(cards); !slices.Equal(got, want) {
		t.Fatalf("partial cards = %v, want %v", got, want)
	}
}

func TestCardsEnglishUnparsableResultCountIsAnError(t *testing.T) {
	listings := map[string]string{
		"1": `<html><body><div class="p_cards__results-box"><ul></ul></div></body></html>`,
	}
	client := newEnglishStubClient(t, nil, listings, okDetailPage)

	// Cards() would hang here if CardsStream returned without closing cardCh.
	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if err == nil {
		t.Fatal("Cards should fail when the result count can't be parsed")
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(cards))
	}
}

func TestCardsEnglishUnsupportedTitleNumberClosesChannel(t *testing.T) {
	client := newEnglishStubClient(t, nil, nil, okDetailPage)

	// Cards() would hang here if CardsStream returned without closing cardCh.
	_, err := client.Cards(context.Background(), Config{Language: Japanese, TitleNumber: 1})
	if err == nil {
		t.Fatal("Cards should reject TitleNumber on the Japanese site")
	}
}

func TestCardsEnglishShortPageWarnsAndIsIncomplete(t *testing.T) {
	// The site says 19 results (2 pages) but page 1 only links 3 cards and
	// page 2 only 1, so both pages and the final total come up short.
	listings := map[string]string{
		"1": englishListingPage(19, "AB/W31-E001", "AB/W31-E002", "AB/W31-E003"),
		"2": englishListingPage(19, "AB/W31-E016"),
	}
	var logs syncBuffer
	client := newEnglishStubClient(t, &logs, listings, okDetailPage)

	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if !strings.Contains(err.Error(), "site reports 19 results but found 4") {
		t.Fatalf("error should describe the count mismatch, got: %v", err)
	}
	if len(cards) != 4 {
		t.Fatalf("expected 4 cards, got %d", len(cards))
	}

	got := logs.String()
	for _, want := range []string{
		`msg="Results page card count mismatch" url="https://en.ws-tcg.com/cardlist/searchresults/?page=1" expected=15 found=3`,
		`msg="Results page card count mismatch" url="https://en.ws-tcg.com/cardlist/searchresults/?page=2" expected=4 found=1`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing log line %q in:\n%s", want, got)
		}
	}
}

func TestCardsEnglishPageStartSkipsCountCheck(t *testing.T) {
	listings := map[string]string{
		"1": englishListingPage(16, "AB/W31-E001"),
		"2": englishListingPage(16, "AB/W31-E016"),
	}
	client := newEnglishStubClient(t, nil, listings, okDetailPage)

	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1, PageStart: 2})
	if err != nil {
		t.Fatalf("Cards failed: %v", err)
	}
	if got := cardNumbers(cards); !slices.Equal(got, []string{"AB/W31-E016"}) {
		t.Fatalf("cards = %v, want only page 2", got)
	}
}

// japaneseSearchPage builds a searchJson response for one page of a
// two-card-per-page result set.
func japaneseSearchPage(total, page int, cardNos ...string) string {
	var items []string
	for i, no := range cardNos {
		items = append(items, `{"id":`+strconv.Itoa(page*10+i)+`,"card_number":"`+no+`","card_name":"n","card_kind":"2","color":"[[yellow.gif]]","level":"0","cost":"0","power":"500","soul":"[[soul.gif]]","card_trigger":"-","text":"t","picture":"x.png","expansion":1,"rare":"RR","side":"-1"}`)
	}
	pageCount := (total + 1) / 2
	return `{"items":[` + strings.Join(items, ",") + `],"total":` + strconv.Itoa(total) + `,"page":` + strconv.Itoa(page) + `,"limit":2,"page_count":` + strconv.Itoa(pageCount) + `}`
}

// newJapaneseStubClient returns a fast client whose transport serves
// searchJson pages by page number; missing pages get a 500. Expansion and
// product lookups are stubbed so no card resolution needs the network.
func newJapaneseStubClient(t *testing.T, logs *syncBuffer, pages map[string]string) *Client {
	t.Helper()
	opts := fastTestOptions()
	if logs != nil {
		opts = append(opts, WithLogger(slog.New(slog.NewTextHandler(logs, nil))))
	}
	client, err := NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(client.Close)

	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Path, "/wordpress/wp-content/images/cardlist/") {
			return newHTTPResponse(r, http.StatusInternalServerError, nil, ""), nil
		}
		switch r.URL.Path {
		case "/manage/CardListUser/filter-options":
			return newHTTPResponse(r, http.StatusOK, nil, `{"expansions":[{"id":1,"name":"X","category":"1","disp_flg":1}]}`), nil
		case "/manage/CardListUser/searchJson":
			if body, ok := pages[r.URL.Query().Get("page")]; ok {
				return newHTTPResponse(r, http.StatusOK, nil, body), nil
			}
			return newHTTPResponse(r, http.StatusInternalServerError, nil, ""), nil
		case "/wp-json/wp/v2/products":
			return newHTTPResponse(r, http.StatusOK, nil, `[]`), nil
		default:
			t.Errorf("unexpected URL: %s", r.URL.String())
			return newHTTPResponse(r, http.StatusNotFound, nil, ""), nil
		}
	})
	return client
}

func TestCardsJapaneseFailedLaterPageIsIncomplete(t *testing.T) {
	// 4 results over 2 pages; page 2 is never served.
	client := newJapaneseStubClient(t, nil, map[string]string{
		"1": japaneseSearchPage(4, 1, "DC/W01-001", "DC/W01-002"),
	})

	cards, err := client.Cards(context.Background(), Config{Language: Japanese, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if !strings.Contains(err.Error(), "fetch search page 2") {
		t.Fatalf("error should name the failed page, got: %v", err)
	}
	if got := cardNumbers(cards); !slices.Equal(got, []string{"DC/W01-001", "DC/W01-002"}) {
		t.Fatalf("partial cards = %v, want page 1", got)
	}
}

func TestCardsJapaneseCountMismatchIsIncomplete(t *testing.T) {
	// The API says 4 results over 2 pages but page 2 only carries one item.
	client := newJapaneseStubClient(t, nil, map[string]string{
		"1": japaneseSearchPage(4, 1, "DC/W01-001", "DC/W01-002"),
		"2": japaneseSearchPage(4, 2, "DC/W01-003"),
	})

	cards, err := client.Cards(context.Background(), Config{Language: Japanese, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if !strings.Contains(err.Error(), "site reports 4 results but found 3") {
		t.Fatalf("error should describe the count mismatch, got: %v", err)
	}
	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}
}

func TestCardsJapaneseFailedFirstPageIsNotIncomplete(t *testing.T) {
	client := newJapaneseStubClient(t, nil, nil)

	cards, err := client.Cards(context.Background(), Config{Language: Japanese, ExpansionNumber: 1})
	if err == nil {
		t.Fatal("Cards should fail when the first page can't be fetched")
	}
	if errors.Is(err, ErrIncomplete) {
		t.Fatalf("nothing was scraped, so err should not be ErrIncomplete: %v", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(cards))
	}
}

func TestSearchValues(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want url.Values
	}{
		{
			name: "en expansion all rarities",
			cfg:  Config{Language: English, ExpansionNumber: 288, GetAllRarities: true},
			want: url.Values{"view": {"text"}, "expansion_name": {"288"}, "parallel": {"0"}},
		},
		{
			name: "en title and set codes base rarity",
			cfg:  Config{Language: English, TitleNumber: 159, SetCode: []string{"BAV/W129", "BD/WE49"}},
			want: url.Values{"view": {"text"}, "title": {"159"}, "parallel": {"1"}, "keyword_or": {"BAV/W129 BD/WE49"}, "keyword_type[]": {"no"}},
		},
		{
			name: "ja expansion and set codes",
			cfg:  Config{Language: Japanese, ExpansionNumber: 100, SetCode: []string{"DD/WE17"}, GetAllRarities: true},
			want: url.Values{"cmd": {"search"}, "show_page_count": {"100"}, "show_small": {"0"}, "expansion": {"100"}, "parallel": {"0"}, "title_number": {"##DD/WE17##"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got, err := searchValues(tt.cfg)
			if err != nil {
				t.Fatalf("searchValues failed: %v", err)
			}
			if got.Encode() != tt.want.Encode() {
				t.Fatalf("searchValues = %v, want %v", got, tt.want)
			}
		})
	}

	if _, _, err := searchValues(Config{Language: Japanese, TitleNumber: 1}); err == nil {
		t.Fatal("expected TitleNumber to be rejected for Japanese")
	}
	if _, _, err := searchValues(Config{Language: SiteLanguage(language.German)}); err == nil {
		t.Fatal("expected an unsupported language to be rejected")
	}
}

// newCountingClient returns a fast client whose transport records every
// request and answers each with respond.
func newCountingClient(t *testing.T, respond func(r *http.Request) *http.Response) (*Client, *atomic.Int32) {
	t.Helper()
	client, err := NewClient(fastTestOptions()...)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(client.Close)

	var requests atomic.Int32
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return respond(r), nil
	})
	return client, &requests
}

func TestCardCountEnglish(t *testing.T) {
	client, requests := newCountingClient(t, func(r *http.Request) *http.Response {
		if r.Method != http.MethodPost || r.URL.String() != "https://en.ws-tcg.com/cardlist/searchresults/?page=1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if got := r.PostForm.Encode(); got != "expansion_name=288&parallel=0&view=text" {
			t.Errorf("form = %q", got)
		}
		return newHTTPResponse(r, http.StatusOK, nil, englishListingPage(1226))
	})

	got, err := client.CardCount(context.Background(), Config{Language: English, ExpansionNumber: 288, GetAllRarities: true})
	if err != nil {
		t.Fatalf("CardCount failed: %v", err)
	}
	if got != 1226 {
		t.Fatalf("CardCount = %d, want 1226", got)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("CardCount made %d requests, want 1", n)
	}
}

func TestCardCountEnglishUnparsableIsAnError(t *testing.T) {
	client, _ := newCountingClient(t, func(r *http.Request) *http.Response {
		return newHTTPResponse(r, http.StatusOK, nil, `<html><body>no results box</body></html>`)
	})

	if _, err := client.CardCount(context.Background(), Config{Language: English, ExpansionNumber: 288}); err == nil {
		t.Fatal("CardCount should fail when the count can't be parsed")
	}
}

func TestCardCountJapanese(t *testing.T) {
	client, requests := newCountingClient(t, func(r *http.Request) *http.Response {
		if r.Method != http.MethodGet || r.URL.Path != "/manage/CardListUser/searchJson" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		q := r.URL.Query()
		if q.Get("expansion") != "100" || q.Get("parallel") != "0" || q.Get("page") != "1" {
			t.Errorf("query = %v", q)
		}
		return newHTTPResponse(r, http.StatusOK, nil, japaneseSearchPage(45, 1, "DD/WE17-01", "DD/WE17-02"))
	})

	got, err := client.CardCount(context.Background(), Config{Language: Japanese, ExpansionNumber: 100, GetAllRarities: true})
	if err != nil {
		t.Fatalf("CardCount failed: %v", err)
	}
	if got != 45 {
		t.Fatalf("CardCount = %d, want 45", got)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("CardCount made %d requests, want 1", n)
	}
}

func TestCardCountRejectsGetRecent(t *testing.T) {
	client, requests := newCountingClient(t, func(r *http.Request) *http.Response {
		return newHTTPResponse(r, http.StatusOK, nil, "")
	})

	if _, err := client.CardCount(context.Background(), Config{Language: English, GetRecent: true}); err == nil {
		t.Fatal("CardCount should reject GetRecent")
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("CardCount made %d requests, want 0", n)
	}
}

func TestBoostersGroupsCardsByRelease(t *testing.T) {
	listings := map[string]string{
		"1": englishListingPage(3, "AB/W31-E001", "AB/W31-E002", "AB/WE10-E01"),
	}
	client := newEnglishStubClient(t, nil, listings, okDetailPage)

	boosters, err := client.Boosters(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if err != nil {
		t.Fatalf("Boosters failed: %v", err)
	}
	if len(boosters) != 2 {
		t.Fatalf("expected 2 boosters, got %d: %v", len(boosters), boosters)
	}
	w31, ok := boosters["W31"]
	if !ok || w31.ReleaseCode != "W31" {
		t.Fatalf("missing W31 booster: %+v", boosters)
	}
	if got := cardNumbers(w31.Cards); !slices.Equal(got, []string{"AB/W31-E001", "AB/W31-E002"}) {
		t.Fatalf("W31 cards = %v", got)
	}
	if got := cardNumbers(boosters["WE10"].Cards); !slices.Equal(got, []string{"AB/WE10-E01"}) {
		t.Fatalf("WE10 cards = %v", got)
	}
}

func TestCardsEnglishNonCardPageIsAnError(t *testing.T) {
	listings := map[string]string{
		"1": englishListingPage(2, "AB/W31-E001", "AB/W31-E002"),
	}
	client := newEnglishStubClient(t, nil, listings, func(cardNo string) (int, string) {
		if cardNo == "AB/W31-E002" {
			// A 200 that isn't a card page, e.g. a bot challenge or error page.
			return http.StatusOK, `<html><body><h1>Access denied</h1></body></html>`
		}
		return okDetailPage(cardNo)
	})

	cards, err := client.Cards(context.Background(), Config{Language: English, ExpansionNumber: 1})
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("Cards err = %v, want ErrIncomplete", err)
	}
	if !strings.Contains(err.Error(), "cardno=AB/W31-E002") || !strings.Contains(err.Error(), "produced no card") {
		t.Fatalf("error should name the bad page, got: %v", err)
	}
	if got := cardNumbers(cards); !slices.Equal(got, []string{"AB/W31-E001"}) {
		t.Fatalf("cards = %v, want only the good card (no blank card)", got)
	}
}

func TestCardsEnglishGetRecentWithNoReleasesIsAnError(t *testing.T) {
	client, _ := newCountingClient(t, func(r *http.Request) *http.Response {
		if r.URL.Path == "/cardlist/" {
			return newHTTPResponse(r, http.StatusOK, nil, `<html><body><p>redesigned landing page</p></body></html>`)
		}
		t.Errorf("unexpected URL: %s", r.URL)
		return newHTTPResponse(r, http.StatusNotFound, nil, "")
	})

	cards, err := client.Cards(context.Background(), Config{Language: English, GetRecent: true})
	if err == nil || !strings.Contains(err.Error(), "no recent releases") {
		t.Fatalf("Cards err = %v, want a no-recent-releases error", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(cards))
	}
}

func TestCardsJapaneseGetRecentWithNoReleasesIsAnError(t *testing.T) {
	// The stub's filter-options lists one expansion without newest_flg.
	client := newJapaneseStubClient(t, nil, nil)

	cards, err := client.Cards(context.Background(), Config{Language: Japanese, GetRecent: true})
	if err == nil || !strings.Contains(err.Error(), "no recent releases") {
		t.Fatalf("Cards err = %v, want a no-recent-releases error", err)
	}
	if len(cards) != 0 {
		t.Fatalf("expected no cards, got %d", len(cards))
	}
}

func TestCardsJapaneseLogsImageFetchFailure(t *testing.T) {
	var logs syncBuffer
	client := newJapaneseStubClient(t, &logs, map[string]string{
		"1": japaneseSearchPage(1, 1, "DC/W01-001"),
	})

	cards, err := client.Cards(context.Background(), Config{Language: Japanese, ExpansionNumber: 1, GetImages: true})
	if err != nil {
		t.Fatalf("Cards failed: %v", err)
	}
	if len(cards) != 1 || cards[0].Image != nil {
		t.Fatalf("expected one card without an image, got %+v", cards)
	}
	if got := logs.String(); !strings.Contains(got, "Problem getting image for DC/W01-001") {
		t.Fatalf("missing image failure log in:\n%s", got)
	}
}
