package fetch

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

const (
	englishPromoListingURL  = "https://en.ws-tcg.com/prcards/"
	japanesePromoListingURL = "https://ws-tcg.com/cardlist/card_pr"
)

type resolvedExpansion struct {
	Code        string
	DisplayName string
	ProductURL  string
	SourceType  ExpansionSourceType
}

type promoListingEntry struct {
	CardNumber  string
	DisplayName string
	Code        string
}

func applyExpansionMetadata(card *Card, meta resolvedExpansion) {
	card.ExpansionSlug = meta.Code
	card.ExpansionProductDisplayName = meta.DisplayName
	card.ExpansionProductURL = meta.ProductURL
	card.ExpansionSourceType = meta.SourceType
}

func extractExpansionMetadata(config siteConfig, mainHTML *goquery.Selection) resolvedExpansion {
	if productMeta, ok := extractProductExpansionMetadata(config, mainHTML); ok {
		return productMeta
	}
	return resolvedExpansion{}
}

func extractProductExpansionMetadata(config siteConfig, mainHTML *goquery.Selection) (resolvedExpansion, bool) {
	productLink := findProductLink(config, mainHTML)
	if productLink == nil {
		return resolvedExpansion{}, false
	}

	productURL := productLink.String()
	productSlug := strings.Trim(path.Base(strings.TrimRight(productLink.Path, "/")), "/")

	return resolvedExpansion{
		Code:        productSlug,
		DisplayName: "",
		ProductURL:  productURL,
		SourceType:  ExpansionSourceTypeProductPage,
	}, true
}

func findProductLink(config siteConfig, mainHTML *goquery.Selection) *url.URL {
	var productURL *url.URL
	mainHTML.Find(".p-cards__cardset-link a[href], .p-cards__cardset-wrapper a[href], .cardset a[href]").EachWithBreak(func(i int, sel *goquery.Selection) bool {
		href, ok := sel.Attr("href")
		if !ok {
			return true
		}
		fullURL, err := joinPath(config.baseURL, href)
		if err != nil {
			return true
		}
		if strings.HasPrefix(strings.TrimRight(fullURL.Path, "/")+"/", "/products/") {
			productURL = fullURL
			return false
		}
		return true
	})
	return productURL
}

func (c *Client) resolvePromoExpansionMetadata(ctx context.Context, lang SiteLanguage, card Card) resolvedExpansion {
	meta, _ := c.resolvePromoExpansionMetadataEntry(ctx, lang, card)
	return meta
}

func (c *Client) resolvePromoExpansionMetadataEntry(ctx context.Context, lang SiteLanguage, card Card) (resolvedExpansion, bool) {
	// Use the parsed release/name as fallback metadata when the promo listing
	// is unavailable or doesn't contain this exact card number.
	meta := resolvedExpansion{
		Code:        card.Release,
		DisplayName: card.ExpansionName,
		ProductURL:  "",
		SourceType:  ExpansionSourceTypePromoListing,
	}

	entries, err := c.promoListingEntries(ctx, lang)
	if err != nil {
		return meta, false
	}

	entry, ok := entries[normalizePromoCardNumber(card.CardNumber)]
	if !ok {
		return meta, false
	}

	if entry.DisplayName != "" {
		meta.DisplayName = entry.DisplayName
	}
	if entry.Code != "" {
		meta.Code = entry.Code
	}
	return meta, true
}

func (c *Client) resolveExpansionMetadata(ctx context.Context, lang SiteLanguage, card Card) resolvedExpansion {
	// Promo listings are authoritative for promo printings. Some promo card
	// detail pages link to the underlying product page, even though the card
	// belongs to the PR distribution listed on the official promo page.
	promoMeta, promoMatch := c.resolvePromoExpansionMetadataEntry(ctx, lang, card)
	if promoMatch {
		return promoMeta
	}
	if card.ExpansionProductURL != "" {
		return c.resolveProductExpansionMetadata(ctx, card)
	}
	return promoMeta
}

func (c *Client) resolveProductExpansionMetadata(ctx context.Context, card Card) resolvedExpansion {
	meta := resolvedExpansion{
		Code:        card.ExpansionSlug,
		DisplayName: card.ExpansionProductDisplayName,
		ProductURL:  card.ExpansionProductURL,
		SourceType:  ExpansionSourceTypeProductPage,
	}
	if card.ExpansionProductURL == "" {
		return meta
	}

	c.productPagesMu.Lock()
	if cached, ok := c.productPages[card.ExpansionProductURL]; ok {
		c.productPagesMu.Unlock()
		if cached.Code == "" {
			cached.Code = meta.Code
		}
		if cached.ProductURL == "" {
			cached.ProductURL = meta.ProductURL
		}
		if cached.SourceType == "" {
			cached.SourceType = meta.SourceType
		}
		return cached
	}
	c.productPagesMu.Unlock()

	fetched, err := c.fetchProductExpansionMetadata(ctx, card.ExpansionProductURL, cardDetailReferer(card))
	if err != nil {
		return meta
	}
	if fetched.Code == "" {
		fetched.Code = meta.Code
	}
	if fetched.DisplayName == "" {
		fetched.DisplayName = meta.DisplayName
	}
	if fetched.ProductURL == "" {
		fetched.ProductURL = meta.ProductURL
	}
	if fetched.SourceType == "" {
		fetched.SourceType = meta.SourceType
	}

	c.productPagesMu.Lock()
	defer c.productPagesMu.Unlock()
	if cached, ok := c.productPages[card.ExpansionProductURL]; ok {
		return cached
	}
	c.productPages[card.ExpansionProductURL] = fetched
	return fetched
}

func (c *Client) fetchProductExpansionMetadata(ctx context.Context, productURL string, referer string) (resolvedExpansion, error) {
	resp, err := c.request(ctx, requestOptions{
		Method:  http.MethodGet,
		URL:     productURL,
		Referer: referer,
	})
	if err != nil {
		return resolvedExpansion{}, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
	if err != nil {
		return resolvedExpansion{}, err
	}

	return resolvedExpansion{
		DisplayName: parseProductDisplayName(productURL, doc),
		ProductURL:  productURL,
		SourceType:  ExpansionSourceTypeProductPage,
	}, nil
}

func (c *Client) promoListingEntries(ctx context.Context, lang SiteLanguage) (map[string]promoListingEntry, error) {
	c.promoListingsMu.Lock()
	if entries, ok := c.promoListings[lang]; ok {
		c.promoListingsMu.Unlock()
		return entries, nil
	}
	c.promoListingsMu.Unlock()

	entries, err := c.fetchPromoListingEntries(ctx, lang)
	if err != nil {
		return nil, err
	}

	c.promoListingsMu.Lock()
	defer c.promoListingsMu.Unlock()
	if existing, ok := c.promoListings[lang]; ok {
		return existing, nil
	}
	c.promoListings[lang] = entries
	return entries, nil
}

func (c *Client) fetchPromoListingEntries(ctx context.Context, lang SiteLanguage) (map[string]promoListingEntry, error) {
	var rawURL string
	switch lang {
	case English:
		rawURL = englishPromoListingURL
	case Japanese:
		return c.fetchJapanesePromoListingEntries(ctx)
	default:
		return nil, fmt.Errorf("unsupported language for promo listing: %v", lang)
	}

	resp, err := c.request(ctx, requestOptions{
		Method: http.MethodGet,
		URL:    rawURL,
	})
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
	if err != nil {
		return nil, err
	}

	switch lang {
	case English:
		return parseEnglishPromoListing(doc), nil
	case Japanese:
		return parseJapanesePromoListing(doc), nil
	default:
		return nil, fmt.Errorf("unsupported language for promo listing: %v", lang)
	}
}

func parseEnglishPromoListing(doc *goquery.Document) map[string]promoListingEntry {
	return parsePromoListingTables(doc, map[string][]string{
		"card":         {"card no."},
		"display_name": {"distribution"},
	})
}

func parseJapanesePromoListing(doc *goquery.Document) map[string]promoListingEntry {
	return parsePromoListingTables(doc, map[string][]string{
		"card":         {"カード番号"},
		"display_name": {"主な配布方法", "配布方法"},
	})
}

func parsePromoListingTables(doc *goquery.Document, headerMatchers map[string][]string) map[string]promoListingEntry {
	entries := make(map[string]promoListingEntry)
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		headers := collectTableHeaders(table)
		cardIdx := findHeaderIndex(headers, headerMatchers["card"]...)
		displayNameIdx := findHeaderIndex(headers, headerMatchers["display_name"]...)
		if cardIdx == -1 || displayNameIdx == -1 {
			return
		}

		table.Find("tr").Each(func(i int, row *goquery.Selection) {
			cells := row.Find("td")
			if cells.Length() == 0 {
				return
			}
			cardNumber := normalizePromoCardNumber(cells.Eq(cardIdx).Text())
			if cardNumber == "" {
				return
			}
			entries[cardNumber] = promoListingEntry{
				CardNumber:  cardNumber,
				DisplayName: normalizeSpace(cells.Eq(displayNameIdx).Text()),
			}
		})
	})
	return entries
}

func collectTableHeaders(table *goquery.Selection) []string {
	var headers []string
	table.Find("tr").First().Find("th").Each(func(i int, th *goquery.Selection) {
		headers = append(headers, normalizeSpace(th.Text()))
	})
	return headers
}

func findHeaderIndex(headers []string, candidates ...string) int {
	for i, header := range headers {
		normalized := strings.ToLower(normalizeSpace(header))
		for _, candidate := range candidates {
			if normalized == strings.ToLower(candidate) {
				return i
			}
		}
	}
	return -1
}

func normalizePromoCardNumber(raw string) string {
	raw = normalizeSpace(raw)
	raw = strings.ReplaceAll(raw, " ", "")
	return sanitizeCardNumber(raw)
}

func normalizeSpace(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func parseProductDisplayName(productURL string, doc *goquery.Document) string {
	var raw string
	switch {
	case strings.Contains(productURL, "en.ws-tcg.com"):
		raw = doc.Find(".p-products__item-detail .p-products__ttl").First().Text()
	case strings.Contains(productURL, "ws-tcg.com"):
		raw = doc.Find(".products__articleName").First().Text()
		if strings.TrimSpace(raw) == "" {
			raw = doc.Find(".entry-content h3").First().Text()
		}
	}
	return cleanProductPageTitle(raw)
}

func cleanProductPageTitle(raw string) string {
	raw = normalizeSpace(raw)
	if raw == "" {
		return ""
	}
	for _, sep := range []string{" | ", " ｜ "} {
		if before, _, ok := strings.Cut(raw, sep); ok {
			raw = before
			break
		}
	}
	return strings.TrimSpace(raw)
}

func cardDetailReferer(card Card) string {
	cardNumber := url.QueryEscape(card.CardNumber)
	switch card.Language {
	case language.English.String():
		return fmt.Sprintf("%s?cardno=%s", siteConfigs[English].cardListURL, cardNumber)
	case language.Japanese.String():
		return fmt.Sprintf("%s?cardno=%s", siteConfigs[Japanese].cardListURL, cardNumber)
	}
	return ""
}
