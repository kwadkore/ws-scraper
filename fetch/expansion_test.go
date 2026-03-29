package fetch

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractDataEnSetsProductExpansionMetadata(t *testing.T) {
	cardHTML := `
<div class="l-subpage__container-inner">
    <h1 class="c-heading__ttl c-heading__ttl-deco">Cards</h1>
    <div class="l-subpage__contents-max u-mt-80 u-mt-60-sp">
      
            

      <div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wordpress/wp-content/images/cardimages/ATLA/BP/ATLA_WX04_001S.png" alt="Aang: The Last Airbender" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">ATLA/WX04-001SP</p>
            <p class="ttl u-mt-14 u-mt-16-sp">Aang: The Last Airbender</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>Avatar: The Last Airbender</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>World of Avatar・Air Nomads</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>SP</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/w.gif" alt="" decoding="async">
                                                    </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wordpress/wp-content/images/partimages/yellow.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>0</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>0</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>1500</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wordpress/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 When this card attacks, if all of your characters are 《World of Avatar》, choose 1 of your characters, and that character gets +2000 power until end of turn.<br>【AUTO】 [(1) Put this card into your waiting room] When your other 《World of Avatar》 character is frontal attacked, you may pay the cost. If you do, return that character to your hand.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>-</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©2023 Viacom International Inc. All Rights Reserved.</p>
          </div>
        </div>
      </div>
      
      <a class="c-article__back" href="/cardlist/">BACK TO Cards</a>
      
            
      <div class="p-cards__cardset-wrapper u-mt-100 u-mt-100-sp">
        <h2 class="c-heading__subttl">Card Set</h2>
        <div class="p-cards__cardset-item u-mt-36 u-mt-50-sp">
                    <p class="date">Jun. 16, 2023</p>
          <p class="ttl">Avatar: The Last Airbender</p>
          <ul class="p-cards__cardset-link">
            <li><a href="/cardlist/searchresults/?expansion=196">Cards</a></li>
                        <li><a href="https://en.ws-tcg.com/products/bp-atla/">Product Page</a></li>
                      </ul>
        </div>
      </div>
      
      
<!--### system-contents ###-->            
    </div>
  </div>
  `

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(cardHTML))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone())
	if card.ExpansionName != "Avatar: The Last Airbender" {
		t.Fatalf("unexpected ExpansionName: %q", card.ExpansionName)
	}
	if card.ExpansionSlug != "bp-atla" {
		t.Fatalf("unexpected ExpansionSlug: %q", card.ExpansionSlug)
	}
	if card.ExpansionProductDisplayName != "" {
		t.Fatalf("unexpected ExpansionProductDisplayName: %q", card.ExpansionProductDisplayName)
	}
	if card.ExpansionProductURL != "https://en.ws-tcg.com/products/bp-atla/" {
		t.Fatalf("unexpected ExpansionProductURL: %q", card.ExpansionProductURL)
	}
	if card.ExpansionSourceType != ExpansionSourceTypeProductPage {
		t.Fatalf("unexpected ExpansionSourceType: %q", card.ExpansionSourceType)
	}
}

func TestResolveProductExpansionMetadataUsesProductPageAndCaches(t *testing.T) {
	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	var requests atomic.Int32
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if r.URL.String() != "https://en.ws-tcg.com/products/bp-atla/" {
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		if got := r.Header.Get("Referer"); got != "https://en.ws-tcg.com/cardlist/?cardno=ATLA%2FWX04-001SP" {
			t.Fatalf("unexpected Referer: %q", got)
		}
		body := `
<div class="p-products__item-wrapper">
              <div class="p-products__item-thumbnail">
                <div class="p-products__item-thumbnail-inner"><img width="2733" height="2456" src="https://en.ws-tcg.com/wordpress/wp-content/uploads/20230207140801/BP-box_alta.png" class="attachment-full size-full" alt="" decoding="async" fetchpriority="high">                </div>
              </div>
              <div class="p-products__item-detail">
                <p class="p-products__category">Booster Pack</p>
                <p class="p-products__ttl">Booster Pack Avatar: The Last Airbender</p>
                <div class="p-products__item-list-wrapper">                  <dl class="p-products__item-list">
                    <dt>Release</dt>
                    <dd>June 16, 2023</dd>
                  </dl>                  <dl class="p-products__item-list">
                    <dt>Card Types</dt>
                    <dd>100 types of cards + 44 types (Parallel)
</dd>
                  </dl>                  <dl class="p-products__item-list">
                    <dt>Others</dt>
                    <dd>PARALLEL CARDS INCLUDED<br>
SP (Special) cards with Unique Hot Stamps and ATR (Avatar Rare)</dd>
                  </dl>                </div>
              </div>
            </div>
			`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})

	baseCard := Card{
		CardNumber:          "ATLA/WX04-001SP",
		ExpansionSlug:       "bp-atla",
		ExpansionProductURL: "https://en.ws-tcg.com/products/bp-atla/",
		Language:            "en",
		ExpansionSourceType: ExpansionSourceTypeProductPage,
	}

	first := client.resolveProductExpansionMetadata(context.Background(), baseCard)
	second := client.resolveProductExpansionMetadata(context.Background(), baseCard)

	if first.DisplayName != "Booster Pack Avatar: The Last Airbender" {
		t.Fatalf("unexpected first DisplayName: %q", first.DisplayName)
	}
	if second.DisplayName != "Booster Pack Avatar: The Last Airbender" {
		t.Fatalf("unexpected second DisplayName: %q", second.DisplayName)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected product page to be fetched once, got %d", requests.Load())
	}
}

func TestParseProductDisplayNameJapaneseUsesLiveStructure(t *testing.T) {
	html := `
<div class="entry-content">
  <h3>ブースターパック「バンドリ！ ガールズバンドパーティ！」Vol.2</h3>
  <p>2019/03/16(Sat) 発売</p>
  <p>〖 タイトル区分：BanG Dream!/ 作品番号：BD〗</p>
</div>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	got := parseProductDisplayName("https://ws-tcg.com/products/gbp2/", doc)
	want := "ブースターパック「バンドリ！ ガールズバンドパーティ！」Vol.2"
	if got != want {
		t.Fatalf("unexpected display name: got %q want %q", got, want)
	}
}

func TestResolvePromoExpansionMetadataEnglish(t *testing.T) {
	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != englishPromoListingURL {
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		body := `
<html><body>
  <table>
    <tr><th>Card No.</th><th>Card Name</th><th>Title</th><th>Distribution</th><th>Release Date</th></tr>
    <tr><td>5HY/W83-PE07</td><td>Always Together, Ichika &amp; Nino &amp; Miku &amp; Yotsuba &amp; Itsuki</td><td>5HY</td><td>Shop Tournaments</td><td>01 / 01 / 2022</td></tr>
  </table>
</body></html>`
		return newHTTPResponse(r, http.StatusOK, nil, body), nil
	})

	meta := client.resolvePromoExpansionMetadata(context.Background(), English, Card{
		CardNumber:    "5HY/W83-PE07",
		ExpansionName: "PR Card 【Weiẞ Side】",
		Release:       "W83",
	})

	if meta.DisplayName != "Shop Tournaments" {
		t.Fatalf("unexpected DisplayName: %q", meta.DisplayName)
	}
	if meta.Code != "W83" {
		t.Fatalf("unexpected Code: %q", meta.Code)
	}
	if meta.ProductURL != "" {
		t.Fatalf("unexpected ProductURL: %q", meta.ProductURL)
	}
	if meta.SourceType != ExpansionSourceTypePromoListing {
		t.Fatalf("unexpected SourceType: %q", meta.SourceType)
	}
}

func TestResolvePromoExpansionMetadataEnglishFallsBackFromEmptyCardFields(t *testing.T) {
	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != englishPromoListingURL {
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		body := `
<html><body>
  <table>
    <tr><th>Card No.</th><th>Card Name</th><th>Title</th><th>Distribution</th><th>Release Date</th></tr>
    <tr><td>5HY/W83-PE07</td><td>Always Together, Ichika &amp; Nino &amp; Miku &amp; Yotsuba &amp; Itsuki</td><td>5HY</td><td>Shop Tournaments</td><td>01 / 01 / 2022</td></tr>
  </table>
</body></html>`
		return newHTTPResponse(r, http.StatusOK, nil, body), nil
	})

	meta := client.resolvePromoExpansionMetadata(context.Background(), English, Card{
		CardNumber:    "5HY/W83-PE07",
		ExpansionName: "PR Card 【Weiẞ Side】",
	})

	if meta.DisplayName != "Shop Tournaments" {
		t.Fatalf("unexpected DisplayName: %q", meta.DisplayName)
	}
	if meta.Code != "" {
		t.Fatalf("unexpected Code: %q", meta.Code)
	}
	if meta.SourceType != ExpansionSourceTypePromoListing {
		t.Fatalf("unexpected SourceType: %q", meta.SourceType)
	}
}

func TestResolvePromoExpansionMetadataJapanese(t *testing.T) {
	client, err := NewClient(WithRespectRobots(false), WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != japanesePromoListingURL {
			t.Fatalf("unexpected URL: %s", r.URL.String())
		}
		body := `
<html><body>
  <table>
    <tr><th>カード番号</th><th>カード名称</th><th>主な配布方法</th><th>ネオスタンダード区分</th></tr>
    <tr><td>BD/W47-P11a</td><td>冬制服 牛込りみ</td><td>『月刊ブシロード』2017年4月号付録</td><td>BanG Dream!</td></tr>
  </table>
</body></html>`
		return newHTTPResponse(r, http.StatusOK, nil, body), nil
	})

	meta := client.resolvePromoExpansionMetadata(context.Background(), Japanese, Card{
		CardNumber:    "BD/W47-P11a",
		ExpansionName: "PRカード【Wサイド】",
		Release:       "W47",
	})

	if meta.DisplayName != "『月刊ブシロード』2017年4月号付録" {
		t.Fatalf("unexpected DisplayName: %q", meta.DisplayName)
	}
	if meta.Code != "W47" {
		t.Fatalf("unexpected Code: %q", meta.Code)
	}
	if meta.ProductURL != "" {
		t.Fatalf("unexpected ProductURL: %q", meta.ProductURL)
	}
	if meta.SourceType != ExpansionSourceTypePromoListing {
		t.Fatalf("unexpected SourceType: %q", meta.SourceType)
	}
}
