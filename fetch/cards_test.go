package fetch

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// func TestGetLastPage(t *testing.T) {
// 	f, err := os.Open("mockws/bd.html")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer f.Close()

// 	doc, err := goquery.NewDocumentFromReader(f)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	last := getLastPage(doc)
// 	if last != 69 {
// 		t.Errorf("%v is not last", last)
// 	}
// }

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
		expansion := task.urlValues.Get("expansion")
		if !slices.Contains(expectedExpansion, expansion) {
			t.Errorf("Did not expect %q expansion", expansion)
		}
	}
}

func TestCardsJapaneseUsesSearchJSONPagination(t *testing.T) {
	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0))
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
	<div class="p-cards__detail u-mt-22 u-mt-40-sp"><p>【CONT】 Nothing.</p></div>
	</div>
</div>`
}

func TestCardsEnglishSurvivesUnclosedImgOnListingPage(t *testing.T) {
	listing, err := os.ReadFile("mockws-en/listing_unclosed_img_mid.html")
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0), WithRequestsPerSecond(1000), WithBurst(100))
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
