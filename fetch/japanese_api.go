package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

const (
	japaneseCardAPIBase          = "https://ws-tcg.com/manage/CardListUser"
	japaneseCardSearchAPI        = japaneseCardAPIBase + "/searchJson"
	japaneseCardFilterOptionsAPI = japaneseCardAPIBase + "/filter-options"
	japanesePRSearchAPI          = "https://ws-tcg.com/manage/pr-card/searchJson"
	japaneseProductsAPI          = "https://ws-tcg.com/wp-json/wp/v2/products"
	japaneseCardImageBase        = "https://ws-tcg.com/wordpress/wp-content/images/cardlist/"
)

var iconTokenRE = regexp.MustCompile(`\[\[([^.\]]+)(?:\.[^\]]+)?\]\]`)

type japaneseFilterOptions struct {
	Sides      []japaneseTitleInfo `json:"sides"`
	Expansions []japaneseExpansion `json:"expansions"`
}

// japaneseTitleInfo is one title family from CardListUser/filter-options.
// The live Japanese deck-rules page loads its title/作品番号 list from this
// same sides array (see theme jquery.rules.js MASTER_API), rather than
// embedding Weiss/Schwarz HTML tables.
type japaneseTitleInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	NameKana    string `json:"name_kana"`
	TitleNumber string `json:"title_number"`
	// Side uses the official cardlist encoding: -1 Weiss, -2 Schwarz, -3 both.
	Side   int `json:"side"`
	DelFlg int `json:"del_flg"`
}

type japaneseExpansion struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Side       string `json:"side"`
	Category   string `json:"category"`
	TitleName  string `json:"title_name"`
	NewestFlg  int    `json:"newest_flg"`
	DispFlg    int    `json:"disp_flg"`
	CreateDate string `json:"create_date"`
}

type japaneseProductSearchEntry struct {
	Slug  string `json:"slug"`
	Link  string `json:"link"`
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
	ProductCats []int `json:"products_cat"`
}

type japaneseCardSearchResponse struct {
	Items     []japaneseCardItem `json:"items"`
	Total     int                `json:"total"`
	Page      int                `json:"page"`
	Limit     int                `json:"limit"`
	PageCount int                `json:"page_count"`
}

type japaneseCardDetailResponse struct {
	Card japaneseCardItem `json:"card"`
}

type japaneseCardItem struct {
	ID           int                   `json:"id"`
	CardNumber   string                `json:"card_number"`
	TitleNumber  string                `json:"title_number"`
	CardName     string                `json:"card_name"`
	CardKind     string                `json:"card_kind"`
	Color        string                `json:"color"`
	Level        string                `json:"level"`
	Cost         string                `json:"cost"`
	Power        string                `json:"power"`
	Soul         string                `json:"soul"`
	CardTrigger  string                `json:"card_trigger"`
	Text         string                `json:"text"`
	Flavor       string                `json:"flavor"`
	Picture      string                `json:"picture"`
	Expansion    int                   `json:"expansion"`
	Rare         string                `json:"rare"`
	Feature1     string                `json:"feature1"`
	Feature2     string                `json:"feature2"`
	Feature3     string                `json:"feature3"`
	Side         string                `json:"side"`
	ExpansionRel *japaneseExpansionRel `json:"expansion_rel"`
}

type japaneseExpansionRel struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	CategoryName string `json:"category_name"`
}

type japanesePRSearchResponse struct {
	Items []japanesePRItem `json:"items"`
	Total int              `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

type japanesePRItem struct {
	CardNumber string `json:"card_number"`
	DistWay    string `json:"dist_way"`
}

func (c *Client) cardsStreamJapanese(ctx context.Context, cfg Config, urlValues url.Values, cardCh chan<- Card) error {
	defer close(cardCh)

	filterOptions, err := c.japaneseFilterOptions(ctx)
	if err != nil {
		return fmt.Errorf("fetch japanese filter options: %w", err)
	}
	expansions := filterOptions.expansionMap()

	var tasks []url.Values
	if cfg.GetRecent {
		for _, v := range recentJapaneseExpansionValues(filterOptions) {
			taskValues := cloneURLValues(urlValues)
			taskValues.Set("expansion", v.Get("expansion"))
			tasks = append(tasks, taskValues)
		}
	} else {
		tasks = append(tasks, cloneURLValues(urlValues))
	}

	for _, taskValues := range tasks {
		firstPage, err := c.fetchJapaneseCardSearch(ctx, taskValues, 1)
		if err != nil {
			return err
		}
		lastPage := firstPage.lastPage()
		for i := 1; i <= lastPage; i++ {
			if i < cfg.PageStart {
				continue
			}

			pageNum := i
			if cfg.Reverse {
				pageNum = lastPage - i + 1
			}

			page := firstPage
			if pageNum != 1 {
				page, err = c.fetchJapaneseCardSearch(ctx, taskValues, pageNum)
				if err != nil {
					return err
				}
			}

			for _, item := range page.Items {
				expansion := expansions[item.Expansion]
				card := cardFromJapaneseAPIItem(siteConfigs[Japanese], item, expansion.Name, c.log())
				if isJapanesePromoCard(card) {
					applyExpansionMetadata(&card, c.resolveExpansionMetadata(ctx, Japanese, card))
				} else {
					applyExpansionMetadata(&card, c.resolveJapaneseProductExpansionMetadata(ctx, card, expansion))
				}
				if cfg.GetImages {
					if img, err := getImageWithClient(ctx, c, card.ImageURL); err == nil {
						card.Image = img
					}
				}
				cardCh <- card
			}
		}
	}

	return nil
}

func (c *Client) fetchJapaneseCardSearch(ctx context.Context, params url.Values, page int) (japaneseCardSearchResponse, error) {
	var out japaneseCardSearchResponse
	resp, err := c.request(ctx, requestOptions{
		Method:  http.MethodGet,
		URL:     withQuery(japaneseCardSearchAPI, params, page),
		Referer: siteConfigs[Japanese].cardListURL,
	})
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return out, fmt.Errorf("parse japanese card search json: %w", err)
	}
	return out, nil
}

func (r japaneseCardSearchResponse) lastPage() int {
	if r.PageCount > 0 {
		return r.PageCount
	}
	if r.Limit > 0 && r.Total > 0 {
		return int(math.Ceil(float64(r.Total) / float64(r.Limit)))
	}
	return 1
}

func (c *Client) japaneseFilterOptions(ctx context.Context) (japaneseFilterOptions, error) {
	c.jpFilterMu.Lock()
	if c.jpFilterOptions != nil {
		options := *c.jpFilterOptions
		c.jpFilterMu.Unlock()
		return options, nil
	}
	c.jpFilterMu.Unlock()

	resp, err := c.request(ctx, requestOptions{
		Method:  http.MethodGet,
		URL:     japaneseCardFilterOptionsAPI,
		Referer: siteConfigs[Japanese].cardListURL,
	})
	if err != nil {
		return japaneseFilterOptions{}, err
	}

	var options japaneseFilterOptions
	if err := json.Unmarshal(resp.Body, &options); err != nil {
		return japaneseFilterOptions{}, fmt.Errorf("parse japanese filter options json: %w", err)
	}

	c.jpFilterMu.Lock()
	defer c.jpFilterMu.Unlock()
	if c.jpFilterOptions == nil {
		c.jpFilterOptions = &options
	}
	return *c.jpFilterOptions, nil
}

func (o japaneseFilterOptions) expansionMap() map[int]japaneseExpansion {
	out := make(map[int]japaneseExpansion, len(o.Expansions))
	for _, exp := range o.Expansions {
		out[exp.ID] = exp
	}
	return out
}

func recentJapaneseExpansionValues(options japaneseFilterOptions) []url.Values {
	expansions := append([]japaneseExpansion(nil), options.Expansions...)
	sort.SliceStable(expansions, func(i, j int) bool {
		return expansions[i].CreateDate > expansions[j].CreateDate
	})

	var out []url.Values
	for _, exp := range expansions {
		if exp.DispFlg != 1 || exp.NewestFlg != 1 {
			continue
		}
		out = append(out, url.Values{
			"show_page_count": {"100"},
			"parallel":        {"0"},
			"expansion":       {strconv.Itoa(exp.ID)},
		})
	}
	return out
}

func cardFromJapaneseAPIItem(config siteConfig, item japaneseCardItem, expansionName string, logger *slog.Logger) Card {
	cardNumber := sanitizeCardNumber(item.CardNumber)
	setID, release, releasePackID, cardID := parseCardNumber(cardNumber, logger)
	if item.ExpansionRel != nil && item.ExpansionRel.Name != "" {
		expansionName = item.ExpansionRel.Name
	}

	card := Card{
		CardNumber:    cardNumber,
		SetID:         setID,
		ExpansionName: expansionName,
		Sides:         sidesFromJapaneseAPI(item.Side),
		Release:       release,
		ReleasePackID: releasePackID,
		ID:            cardID,
		Language:      language.Japanese.String(),
		Type:          cardTypeFromJapaneseAPI(item.CardKind),
		Name:          normalizeSpace(item.CardName),
		Level:         parseNumericStat(item.Level),
		Cost:          parseNumericStat(item.Cost),
		FlavorText:    cleanOptionalText(item.Flavor),
		Color:         colorFromIconString(item.Color),
		Power:         parseNumericStat(item.Power),
		Rarity:        normalizeSpace(item.Rare),
		Text:          splitJapaneseAPIText(item.Text),
		Triggers:      triggersFromIconString(item.CardTrigger, cardNumber),
		Traits:        traitsFromJapaneseAPI(item.Feature1, item.Feature2, item.Feature3),
	}
	if card.Type == CardTypeCharacter {
		card.Soul = soulFromIconString(item.Soul)
	}
	if fullURL, err := joinPath(config.baseURL, japaneseCardImageBase+item.Picture); err == nil {
		card.ImageURL = fullURL.String()
	} else {
		card.ImageURL = japaneseCardImageBase + item.Picture
	}
	return card
}

func (c *Client) resolveJapaneseProductExpansionMetadata(ctx context.Context, card Card, expansion japaneseExpansion) resolvedExpansion {
	if expansion.ID == 0 {
		return resolvedExpansion{
			Code:        card.ExpansionSlug,
			DisplayName: card.ExpansionProductDisplayName,
			ProductURL:  card.ExpansionProductURL,
			SourceType:  card.ExpansionSourceType,
		}
	}

	c.jpProductsMu.Lock()
	if cached, ok := c.jpProducts[expansion.ID]; ok {
		c.jpProductsMu.Unlock()
		return cached
	}
	c.jpProductsMu.Unlock()

	meta := resolvedExpansion{
		DisplayName: expansion.Name,
		SourceType:  ExpansionSourceTypeProductPage,
	}

	product, ok := c.findJapaneseProductPost(ctx, expansion)
	if ok {
		meta.Code = strings.TrimSpace(product.Slug)
		meta.ProductURL = strings.TrimSpace(product.Link)
		if meta.DisplayName == "" {
			meta.DisplayName = cleanProductPageTitle(product.Title.Rendered)
		}
	}
	if meta.ProductURL != "" {
		fetched := c.resolveProductExpansionMetadata(ctx, Card{
			CardNumber:                  card.CardNumber,
			Language:                    card.Language,
			ExpansionSlug:               meta.Code,
			ExpansionProductDisplayName: meta.DisplayName,
			ExpansionProductURL:         meta.ProductURL,
			ExpansionSourceType:         ExpansionSourceTypeProductPage,
		})
		if fetched.Code != "" {
			meta.Code = fetched.Code
		}
		if fetched.DisplayName != "" {
			meta.DisplayName = fetched.DisplayName
		}
		if fetched.ProductURL != "" {
			meta.ProductURL = fetched.ProductURL
		}
		if fetched.SourceType != "" {
			meta.SourceType = fetched.SourceType
		}
	}

	c.jpProductsMu.Lock()
	defer c.jpProductsMu.Unlock()
	if cached, ok := c.jpProducts[expansion.ID]; ok {
		return cached
	}
	c.jpProducts[expansion.ID] = meta
	return meta
}

func (c *Client) findJapaneseProductPost(ctx context.Context, expansion japaneseExpansion) (japaneseProductSearchEntry, bool) {
	if strings.TrimSpace(expansion.Name) == "" {
		return japaneseProductSearchEntry{}, false
	}

	values := url.Values{
		"search":  {expansion.Name},
		"_fields": {"link,title,slug,products_cat"},
	}
	resp, err := c.request(ctx, requestOptions{
		Method:  http.MethodGet,
		URL:     japaneseProductsAPI + "?" + values.Encode(),
		Referer: siteConfigs[Japanese].cardListURL,
	})
	if err != nil {
		return japaneseProductSearchEntry{}, false
	}

	var products []japaneseProductSearchEntry
	if err := json.Unmarshal(resp.Body, &products); err != nil {
		return japaneseProductSearchEntry{}, false
	}
	if len(products) == 0 {
		return japaneseProductSearchEntry{}, false
	}

	targetCat, hasTargetCat := japaneseProductCategoryID(expansion.Category)
	best := products[0]
	bestScore := japaneseProductMatchScore(best, expansion.Name, targetCat, hasTargetCat)
	for _, product := range products[1:] {
		score := japaneseProductMatchScore(product, expansion.Name, targetCat, hasTargetCat)
		if score > bestScore {
			best = product
			bestScore = score
		}
	}
	if best.Link == "" {
		return japaneseProductSearchEntry{}, false
	}
	return best, true
}

func japaneseProductMatchScore(product japaneseProductSearchEntry, expansionName string, targetCat int, hasTargetCat bool) int {
	score := 0
	title := cleanProductPageTitle(product.Title.Rendered)
	if title == expansionName {
		score += 10
	}
	if strings.Contains(title, expansionName) {
		score += 5
	}
	if hasTargetCat {
		for _, cat := range product.ProductCats {
			if cat == targetCat {
				score += 20
				break
			}
		}
	}
	return score
}

func japaneseProductCategoryID(category string) (int, bool) {
	switch strings.TrimSpace(category) {
	case "1":
		return 15, true // booster-pack
	case "2":
		return 17, true // extra-booster
	case "3":
		return 14, true // trial-deck
	case "4":
		return 18, true // others
	case "6":
		return 16, true // premium-booster
	default:
		return 0, false
	}
}

func isJapanesePromoCard(card Card) bool {
	if strings.EqualFold(card.Rarity, "PR") {
		return true
	}
	return strings.Contains(strings.ToUpper(card.ID), "P")
}

func sidesFromJapaneseAPI(raw string) []Side {
	switch strings.TrimSpace(raw) {
	case "-1":
		return []Side{SideWeiss}
	case "-2":
		return []Side{SideSchwarz}
	case "-3":
		return []Side{SideWeiss, SideSchwarz}
	default:
		return nil
	}
}

func cardTypeFromJapaneseAPI(raw string) CardType {
	switch strings.TrimSpace(raw) {
	case "2":
		return CardTypeCharacter
	case "3":
		return CardTypeEvent
	case "4":
		return CardTypeClimax
	default:
		return ""
	}
}

func colorFromIconString(raw string) CardColor {
	switch firstIconName(raw) {
	case "blue":
		return CardColorBlue
	case "green":
		return CardColorGreen
	case "red":
		return CardColorRed
	case "yellow":
		return CardColorYellow
	case "purple":
		return CardColorPurple
	default:
		if color, ok := jpTextColorMap[normalizeSpace(raw)]; ok {
			return color
		}
		return CardColor(strings.ToUpper(normalizeSpace(raw)))
	}
}

func soulFromIconString(raw string) *int {
	count := 0
	for _, name := range iconNames(raw) {
		if name == "soul" {
			count++
		}
	}
	if count == 0 {
		return parseNumericStat(raw)
	}
	return &count
}

func triggersFromIconString(raw string, cardNumber string) []Trigger {
	var triggers []Trigger
	for _, name := range iconNames(raw) {
		trigger, ok := triggersMap[name]
		if !ok {
			continue
		}
		triggers = append(triggers, trigger)
	}
	return triggers
}

func iconNames(raw string) []string {
	var names []string
	for _, match := range iconTokenRE.FindAllStringSubmatch(raw, -1) {
		names = append(names, strings.ToLower(strings.TrimSpace(match[1])))
	}
	return names
}

func firstIconName(raw string) string {
	names := iconNames(raw)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func traitsFromJapaneseAPI(features ...string) []string {
	var traits []string
	for _, feature := range features {
		feature = cleanOptionalText(feature)
		if feature == "" {
			continue
		}
		parts := strings.Split(feature, "・")
		for _, part := range parts {
			part = cleanOptionalText(part)
			if part != "" {
				traits = append(traits, part)
			}
		}
	}
	return traits
}

func splitJapaneseAPIText(raw string) []string {
	raw = strings.ReplaceAll(raw, "<br />", "\n")
	raw = strings.ReplaceAll(raw, "<br/>", "\n")
	raw = strings.ReplaceAll(raw, "<br>", "\n")
	raw = html.UnescapeString(raw)
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(raw)))
	if err == nil {
		raw = doc.Text()
	}

	var out []string
	for _, part := range strings.Split(raw, "\n") {
		part = cleanOptionalText(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cleanOptionalText(raw string) string {
	raw = normalizeSpace(html.UnescapeString(raw))
	if raw == "" || regexp.MustCompile(`^[-－ー—―]+$`).MatchString(raw) {
		return ""
	}
	return raw
}

func cloneURLValues(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for k, v := range values {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func withQuery(rawURL string, params url.Values, page int) string {
	values := cloneURLValues(params)
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if values.Get("show_page_count") != "" && values.Get("limit") == "" {
		values.Set("limit", values.Get("show_page_count"))
	}
	if len(values) == 0 {
		return rawURL
	}
	return rawURL + "?" + values.Encode()
}

func (c *Client) fetchJapanesePromoListingEntries(ctx context.Context) (map[string]promoListingEntry, error) {
	entries := make(map[string]promoListingEntry)
	for page := 1; ; page++ {
		params := url.Values{"limit": {"100"}}
		resp, err := c.request(ctx, requestOptions{
			Method:  http.MethodGet,
			URL:     withQuery(japanesePRSearchAPI, params, page),
			Referer: japanesePromoListingURL,
		})
		if err != nil {
			return nil, err
		}

		var parsed japanesePRSearchResponse
		if err := json.Unmarshal(resp.Body, &parsed); err != nil {
			return nil, fmt.Errorf("parse japanese pr listing json: %w", err)
		}
		for _, item := range parsed.Items {
			cardNumber := normalizePromoCardNumber(item.CardNumber)
			if cardNumber == "" {
				continue
			}
			entries[cardNumber] = promoListingEntry{
				CardNumber:  cardNumber,
				DisplayName: normalizeSpace(item.DistWay),
			}
		}

		if parsed.Limit <= 0 || page*parsed.Limit >= parsed.Total || len(parsed.Items) == 0 {
			break
		}
	}
	return entries, nil
}
