package fetch

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	englishDeckRulesURL  = "https://en.ws-tcg.com/rules/deck/"
	japaneseDeckRulesURL = "https://ws-tcg.com/rules/deck_rule/"
)

var (
	englishChooseOneOfRE  = regexp.MustCompile(`Choose 1 of (\d+)`)
	englishUpToCopiesRE   = regexp.MustCompile(`Up to (\d+) copies`)
	japaneseChooseOneOfRE = regexp.MustCompile(`(\d+)種選抜`)
	japaneseUpToCopiesRE  = regexp.MustCompile(`(\d+)枚まで使用可`)
)

type DeckRulesConfig struct {
	// Language selects which official site to parse.
	Language SiteLanguage
}

// RestrictionType classifies a deck-building rule applied to one or more cards.
type RestrictionType string

const (
	RestrictionTypeUnrestricted      RestrictionType = "unrestricted"
	RestrictionTypeRestricted        RestrictionType = "restricted"
	RestrictionTypeLimited           RestrictionType = "limited"
	RestrictionTypeChoiceRestriction RestrictionType = "choice_restriction"
	RestrictionTypeAnyCombination    RestrictionType = "any_combination"
)

// TitleDeckGroup describes one side-specific deck-construction grouping from
// the official rules pages.
//
// A group represents the set/title codes that may be mixed together for one
// title family on one side. CanonicalName is the normalized family name used by
// this scraper, while Aliases preserves alternate labels that should resolve to
// the same family. AllowedCodes are not globally unique: the same code may
// legally appear in multiple groups if the official rules list it that way.
type TitleDeckGroup struct {
	// CanonicalName is the normalized title family name for this group.
	CanonicalName string `json:"canonicalName"`
	// Side is the side this group belongs to: "W" for Weiss or "S" for Schwarz.
	Side string `json:"side"`
	// AllowedCodes are the set/title codes that may be used in this group.
	AllowedCodes []string `json:"allowedCodes"`
	// Aliases contains alternate labels for the same title family.
	Aliases []string `json:"aliases"`
	// SourceLanguage identifies which language site the row was parsed from.
	SourceLanguage string `json:"sourceLanguage"`
	// SourceTitle preserves the original title text from the source row.
	SourceTitle string `json:"sourceTitle"`
	// SourceURL is the rules page that produced this group.
	SourceURL string `json:"sourceURL"`
	// Notes contains parser-preserved context from the source row when useful,
	// such as side-selection notes from the Japanese rules page.
	Notes []string `json:"notes,omitempty"`
}

// RestrictionCard describes one named card entry within a restriction group.
// CardNumbers may contain multiple printings that share the same rule.
type RestrictionCard struct {
	Name        string   `json:"name"`
	CardNumbers []string `json:"cardNumbers"`
	IsNew       bool     `json:"isNew,omitempty"`
}

// FreeFloaterGroup describes cards that can be used across otherwise separate
// deck groupings, such as the English "any format" list.
type FreeFloaterGroup struct {
	Label          string            `json:"label"`
	Cards          []RestrictionCard `json:"cards"`
	Notes          []string          `json:"notes,omitempty"`
	SourceLanguage string            `json:"sourceLanguage"`
	SourceURL      string            `json:"sourceURL"`
}

// RestrictionGroup describes one normalized restriction rule parsed from the
// official rules pages.
type RestrictionGroup struct {
	// Type is the normalized restriction category.
	Type RestrictionType `json:"type"`
	// Label preserves the original restriction label text from the source page.
	Label string `json:"label"`
	// Updated reports whether the source label marked this rule as updated.
	Updated bool `json:"updated,omitempty"`
	// MaxCopies is the maximum number of copies allowed by this rule, when one
	// is explicitly defined.
	MaxCopies int `json:"maxCopies,omitempty"`
	// ChooseOneOf is the size of a choice-restriction set, when specified.
	ChooseOneOf int `json:"chooseOneOf,omitempty"`
	// Cards are the card entries covered by this rule.
	Cards []RestrictionCard `json:"cards"`
	// Notes contains parser-preserved notes attached to this rule.
	Notes []string `json:"notes,omitempty"`
	// SourceLanguage identifies which language site the rule was parsed from.
	SourceLanguage string `json:"sourceLanguage"`
	// SourceURL is the rules page that produced this rule.
	SourceURL string `json:"sourceURL"`
}

// DeckRules contains the parsed deck-rules data for one language,
// including title groups and card-level restrictions.
type DeckRules struct {
	TitleDeckGroups     []TitleDeckGroup              `json:"titleDeckGroups"`
	FreeFloaters        []FreeFloaterGroup            `json:"freeFloaters,omitempty"`
	RestrictionsByGroup map[string][]RestrictionGroup `json:"restrictionsByGroup,omitempty"`
}

// DeckRules fetches and parses the official deck-construction rules page for
// the requested language and returns normalized title groups plus card-level
// restriction data.
func (c *Client) DeckRules(ctx context.Context, cfg DeckRulesConfig) (DeckRules, error) {
	switch cfg.Language {
	case English:
		doc, err := c.getDocument(ctx, englishDeckRulesURL, "")
		if err != nil {
			return DeckRules{}, err
		}
		return parseEnglishDeckRules(doc)
	case Japanese:
		doc, err := c.getDocument(ctx, japaneseDeckRulesURL, "")
		if err != nil {
			return DeckRules{}, err
		}
		return parseJapaneseDeckRules(doc)
	default:
		return DeckRules{}, fmt.Errorf("unsupported language: %v", cfg.Language)
	}
}

// DeckConstruction fetches and parses the official deck-construction rules page
// for the requested language and returns normalized, side-aware title groups.
func (c *Client) DeckConstruction(ctx context.Context, cfg DeckRulesConfig) ([]TitleDeckGroup, error) {
	rules, err := c.DeckRules(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return rules.TitleDeckGroups, nil
}

func parseEnglishDeckRules(doc *goquery.Document) (DeckRules, error) {
	titleGroups, err := parseEnglishDeckConstruction(doc)
	if err != nil {
		return DeckRules{}, err
	}
	freeFloaters, restrictions, err := parseEnglishRestrictions(doc)
	if err != nil {
		return DeckRules{}, err
	}
	return DeckRules{
		TitleDeckGroups:     titleGroups,
		FreeFloaters:        freeFloaters,
		RestrictionsByGroup: groupRestrictionsByName(restrictions),
	}, nil
}

func parseEnglishDeckConstruction(doc *goquery.Document) ([]TitleDeckGroup, error) {
	tables := doc.Find("table.title-list")
	if tables.Length() < 2 {
		return nil, fmt.Errorf("couldn't find English title tables")
	}

	var groups []TitleDeckGroup
	tables.Each(func(i int, table *goquery.Selection) {
		side := "W"
		if i == 1 {
			side = "S"
		}
		table.Find("tr").Each(func(rowIdx int, tr *goquery.Selection) {
			if rowIdx == 0 {
				return
			}
			cells := tr.Find("td")
			if cells.Length() < 2 {
				return
			}
			title := extractTitleCell(cells.Eq(0))
			if title == "" {
				return
			}
			codes := splitCodes(selectionTextWithLineBreaks(cells.Eq(1)))
			if len(codes) == 0 {
				return
			}
			groups = append(groups, normalizeTitleDeckGroup(titleDeckRow{
				side:      side,
				title:     title,
				codes:     codes,
				sourceURL: englishDeckRulesURL,
				language:  English.String(),
			}))
		})
	})

	if len(groups) == 0 {
		return nil, fmt.Errorf("no English title rows parsed")
	}
	return groups, nil
}

func parseJapaneseDeckRules(doc *goquery.Document) (DeckRules, error) {
	titleGroups, err := parseJapaneseDeckConstruction(doc)
	if err != nil {
		return DeckRules{}, err
	}
	freeFloaters, restrictions, err := parseJapaneseRestrictions(doc)
	if err != nil {
		return DeckRules{}, err
	}
	return DeckRules{
		TitleDeckGroups:     titleGroups,
		FreeFloaters:        freeFloaters,
		RestrictionsByGroup: groupRestrictionsByName(restrictions),
	}, nil
}

func parseJapaneseDeckConstruction(doc *goquery.Document) ([]TitleDeckGroup, error) {
	entry := doc.Find(".entry-content").First()
	if entry.Length() == 0 {
		return nil, fmt.Errorf("couldn't find Japanese rules content")
	}

	var tables []*goquery.Selection
	entry.Find("table").Each(func(i int, table *goquery.Selection) {
		header := normalizeWhitespace(table.Find("tr").First().Text())
		switch {
		case strings.Contains(header, "タイトル（ヴァイスサイド）"):
			tables = append(tables, table)
		case strings.Contains(header, "タイトル（シュヴァルツサイド）"):
			tables = append(tables, table)
		}
	})
	if len(tables) < 2 {
		return nil, fmt.Errorf("couldn't find Japanese title tables")
	}

	var groups []TitleDeckGroup
	for _, table := range tables {
		header := normalizeWhitespace(table.Find("tr").First().Text())
		side := "W"
		sideNote := "ヴァイスサイド"
		if strings.Contains(header, "シュヴァルツサイド") {
			side = "S"
			sideNote = "シュヴァルツサイド"
		}
		table.Find("tr").Each(func(rowIdx int, tr *goquery.Selection) {
			if rowIdx == 0 {
				return
			}
			cells := tr.Find("td")
			if cells.Length() < 2 {
				return
			}
			title := extractTitleCell(cells.Eq(0))
			if title == "" {
				return
			}
			codes, notes := parseJapaneseCodes(cells.Eq(1), sideNote)
			if len(codes) == 0 {
				return
			}
			groups = append(groups, normalizeTitleDeckGroup(titleDeckRow{
				side:      side,
				title:     title,
				codes:     codes,
				notes:     notes,
				sourceURL: japaneseDeckRulesURL,
				language:  Japanese.String(),
			}))
		})
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no Japanese title rows parsed")
	}
	return groups, nil
}

type titleDeckRow struct {
	side      string
	title     string
	codes     []string
	notes     []string
	sourceURL string
	language  string
}

func normalizeTitleDeckGroup(row titleDeckRow) TitleDeckGroup {
	canonicalName, aliases := canonicalizeTitle(row.title)

	return TitleDeckGroup{
		CanonicalName:  canonicalName,
		Side:           row.side,
		AllowedCodes:   uniquePreserveOrder(row.codes),
		Aliases:        uniquePreserveOrder(aliases),
		SourceLanguage: row.language,
		SourceTitle:    row.title,
		SourceURL:      row.sourceURL,
		Notes:          uniquePreserveOrder(row.notes),
	}
}

func splitAliases(raw string) []string {
	parts := strings.Split(raw, ",")
	if len(parts) == 1 {
		return []string{normalizeWhitespace(raw)}
	}
	var aliases []string
	for _, part := range parts {
		part = normalizeWhitespace(part)
		if part != "" {
			aliases = append(aliases, part)
		}
	}
	return aliases
}

func canonicalizeTitle(raw string) (string, []string) {
	aliases := splitAliases(raw)
	if len(aliases) <= 1 {
		return raw, nil
	}

	if base, ok := findSharedBaseAlias(aliases); ok {
		var otherAliases []string
		for _, alias := range aliases {
			if alias == base {
				continue
			}
			otherAliases = append(otherAliases, alias)
		}
		return base, otherAliases
	}

	return raw, aliases
}

func findSharedBaseAlias(aliases []string) (string, bool) {
	for _, candidate := range aliases {
		if candidate == "" {
			continue
		}
		matchesAll := true
		for _, alias := range aliases {
			if !titleAliasMatchesBase(alias, candidate) {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			return candidate, true
		}
	}
	return "", false
}

func titleAliasMatchesBase(alias string, base string) bool {
	if alias == base {
		return true
	}
	if !strings.HasPrefix(alias, base) {
		return false
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(alias, base))
	if remainder == "" {
		return true
	}
	return strings.HasPrefix(alias, base+" ") ||
		strings.HasPrefix(alias, base+"[") ||
		strings.HasPrefix(alias, base+"(") ||
		strings.HasPrefix(alias, base+"-") ||
		strings.HasPrefix(alias, base+":")
}

func extractTitleCell(cell *goquery.Selection) string {
	text := selectionTextWithLineBreaks(cell)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = normalizeWhitespace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "※") {
			continue
		}
		return line
	}
	return ""
}

func parseJapaneseCodes(cell *goquery.Selection, sideLabel string) ([]string, []string) {
	lines := strings.Split(selectionTextWithLineBreaks(cell), "\n")
	var mainLines []string
	var subsetLines []string
	var noteLines []string
	inSubset := false

	for _, line := range lines {
		line = normalizeWhitespace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "※このうち、下記の作品番号が") {
			inSubset = true
			noteLines = append(noteLines, line)
			continue
		}
		if inSubset {
			if strings.Contains(line, sideLabel+"として扱われます") {
				noteLines = append(noteLines, line)
				continue
			}
			subsetLines = append(subsetLines, line)
			continue
		}
		mainLines = append(mainLines, line)
	}

	mainCodes := splitCodes(strings.Join(mainLines, ","))
	subsetCodes := splitCodes(strings.Join(subsetLines, ","))
	if len(subsetCodes) == 0 {
		return mainCodes, noteLines
	}

	notes := append([]string{}, noteLines...)
	if joined := strings.Join(mainCodes, ","); joined != "" {
		notes = append(notes, "full code list: "+joined)
	}
	return subsetCodes, notes
}

func parseEnglishRestrictions(doc *goquery.Document) ([]FreeFloaterGroup, []RestrictionGroup, error) {
	var freeFloaters []FreeFloaterGroup
	var groups []RestrictionGroup

	unrestrictedTable := doc.Find("p").FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.Contains(normalizeWhitespace(s.Text()), "You may use the following cards in any format")
	}).First().NextAllFiltered("table.title-list").First()
	if unrestrictedTable.Length() == 0 {
		return nil, nil, fmt.Errorf("couldn't find English unrestricted card table")
	}

	unrestricted := FreeFloaterGroup{
		Label:          "You may use the following cards in any format, unless stated otherwise.",
		SourceLanguage: English.String(),
		SourceURL:      englishDeckRulesURL,
	}
	unrestrictedTable.Find("tr").Each(func(i int, tr *goquery.Selection) {
		if i == 0 {
			return
		}
		cells := tr.Find("td")
		if cells.Length() < 2 {
			return
		}
		cardNumber := normalizeWhitespace(cells.Eq(0).Text())
		cardName := normalizeWhitespace(cells.Eq(1).Text())
		if cardNumber == "" || cardName == "" {
			return
		}
		unrestricted.Cards = append(unrestricted.Cards, RestrictionCard{
			Name:        cardName,
			CardNumbers: []string{cardNumber},
		})
	})
	if len(unrestricted.Cards) > 0 {
		freeFloaters = append(freeFloaters, unrestricted)
	}

	doc.Find("table.restriction-list").Each(func(i int, table *goquery.Selection) {
		groupName := normalizeWhitespace(table.Find("th").First().Text())
		if groupName == "" {
			return
		}
		currentIndex := -1
		table.Find("tr").Each(func(rowIdx int, tr *goquery.Selection) {
			if rowIdx == 0 {
				return
			}
			cells := tr.Find("td")
			switch cells.Length() {
			case 3:
				label := normalizeWhitespace(selectionTextWithLineBreaks(cells.Eq(0)))
				restriction := parseRestrictionLabel(label, English.String(), englishDeckRulesURL)
				restriction.Cards = append(restriction.Cards, RestrictionCard{
					Name:        normalizeMarkerText(normalizeWhitespace(cells.Eq(1).Text())),
					CardNumbers: extractCardNumbers(cells.Eq(2)),
					IsNew:       containsNewMarker(selectionTextWithLineBreaks(cells.Eq(1))),
				})
				restriction.Notes = append(restriction.Notes, "groupName="+groupName)
				groups = append(groups, restriction)
				currentIndex = len(groups) - 1
			case 2:
				if currentIndex < 0 {
					return
				}
				groups[currentIndex].Cards = append(groups[currentIndex].Cards, RestrictionCard{
					Name:        normalizeMarkerText(normalizeWhitespace(cells.Eq(0).Text())),
					CardNumbers: extractCardNumbers(cells.Eq(1)),
					IsNew:       containsNewMarker(selectionTextWithLineBreaks(cells.Eq(0))),
				})
			case 1:
				if currentIndex < 0 {
					return
				}
				note := normalizeWhitespace(selectionTextWithLineBreaks(cells.Eq(0)))
				if note != "" {
					groups[currentIndex].Notes = append(groups[currentIndex].Notes, note)
				}
			}
		})
	})

	if len(groups) == 0 {
		return nil, nil, fmt.Errorf("no English restriction data parsed")
	}
	return freeFloaters, attachRestrictionGroupNotes(groups), nil
}

func parseJapaneseRestrictions(doc *goquery.Document) ([]FreeFloaterGroup, []RestrictionGroup, error) {
	entry := doc.Find(".entry-content").First()
	if entry.Length() == 0 {
		return nil, nil, fmt.Errorf("couldn't find Japanese rules content")
	}

	var freeFloaters []FreeFloaterGroup
	var groups []RestrictionGroup

	// TODO: Parse the separate "タイトル限定構築特例カード一覧" compatibility table.
	// It is intentionally excluded from the returned client-facing data for now.

	entry.Find("table").EachWithBreak(func(i int, table *goquery.Selection) bool {
		if !strings.Contains(normalizeWhitespace(table.Text()), "すべてのタイトルで使用可能") {
			return true
		}
		unrestricted := FreeFloaterGroup{
			Label:          "すべてのタイトルで使用可能",
			SourceLanguage: Japanese.String(),
			SourceURL:      japaneseDeckRulesURL,
		}
		inAllTitlesSection := false
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			cells := tr.Find("td")
			if cells.Length() == 0 {
				return
			}
			if cells.Length() == 1 && cells.First().AttrOr("colspan", "") == "2" {
				header := normalizeWhitespace(cells.First().Text())
				if header == "すべてのタイトルで使用可能" {
					inAllTitlesSection = true
					return
				}
				if inAllTitlesSection {
					inAllTitlesSection = false
				}
				return
			}
			if !inAllTitlesSection || cells.Length() < 2 {
				return
			}
			cardNumbers := extractCardNumbers(cells.Eq(0))
			cardName := normalizeWhitespace(cells.Eq(1).Text())
			if len(cardNumbers) == 0 || cardName == "" {
				return
			}
			unrestricted.Cards = append(unrestricted.Cards, RestrictionCard{
				Name:        cardName,
				CardNumbers: cardNumbers,
			})
		})
		if len(unrestricted.Cards) > 0 {
			freeFloaters = append(freeFloaters, unrestricted)
		}
		return false
	})

	var restrictionTable *goquery.Selection
	entry.Find("details").EachWithBreak(func(i int, details *goquery.Selection) bool {
		summary := normalizeWhitespace(details.Find("summary").Text())
		if !strings.Contains(summary, "ネオスタンダード構築／タイトル限定構築") {
			return true
		}
		tables := details.Find("table[border='1']")
		if tables.Length() == 0 {
			return true
		}
		restrictionTable = tables.Last()
		return false
	})
	if restrictionTable == nil || restrictionTable.Length() == 0 {
		return nil, nil, fmt.Errorf("couldn't find Japanese restriction table")
	}

	currentTitle := ""
	currentIndex := -1
	restrictionTable.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		cells := tr.Find("td")
		if cells.Length() == 0 {
			return
		}
		if cells.Length() == 1 && cells.First().AttrOr("colspan", "") == "3" {
			currentTitle = normalizeWhitespace(cells.First().Text())
			currentIndex = -1
			return
		}
		switch cells.Length() {
		case 3:
			if currentTitle == "" {
				return
			}
			label := normalizeWhitespace(selectionTextWithLineBreaks(cells.Eq(0)))
			restriction := parseRestrictionLabel(label, Japanese.String(), japaneseDeckRulesURL)
			restriction.Cards = append(restriction.Cards, RestrictionCard{
				Name:        normalizeMarkerText(normalizeWhitespace(cells.Eq(1).Text())),
				CardNumbers: extractCardNumbers(cells.Eq(2)),
			})
			restriction.Notes = append(restriction.Notes, "groupName="+currentTitle)
			groups = append(groups, restriction)
			currentIndex = len(groups) - 1
		case 2:
			if currentIndex < 0 {
				return
			}
			groups[currentIndex].Cards = append(groups[currentIndex].Cards, RestrictionCard{
				Name:        normalizeMarkerText(normalizeWhitespace(cells.Eq(0).Text())),
				CardNumbers: extractCardNumbers(cells.Eq(1)),
			})
		}
	})

	if len(groups) == 0 {
		return nil, nil, fmt.Errorf("no Japanese restriction data parsed")
	}
	return freeFloaters, attachRestrictionGroupNotes(groups), nil
}

func parseRestrictionLabel(label string, language string, sourceURL string) RestrictionGroup {
	cleanLabel := normalizeMarkerText(label)
	restriction := RestrictionGroup{
		Label:          cleanLabel,
		Updated:        containsUpdatedMarker(label),
		SourceLanguage: language,
		SourceURL:      sourceURL,
	}
	lowerLabel := strings.ToLower(cleanLabel)

	switch {
	case strings.Contains(lowerLabel, "cannot be used") || strings.Contains(cleanLabel, "使用不可"):
		restriction.Type = RestrictionTypeRestricted
	case strings.Contains(lowerLabel, "any combination"):
		restriction.Type = RestrictionTypeAnyCombination
		restriction.MaxCopies = parseFirstInt(cleanLabel)
	case strings.Contains(lowerLabel, "choice restriction") || strings.Contains(cleanLabel, "種選抜"):
		restriction.Type = RestrictionTypeChoiceRestriction
		restriction.ChooseOneOf = parseChooseOneOf(cleanLabel)
		restriction.MaxCopies = parseMaxCopies(cleanLabel)
		if restriction.MaxCopies == 0 {
			restriction.MaxCopies = 4
		}
	case strings.Contains(lowerLabel, "limited") || strings.Contains(cleanLabel, "枚まで使用可"):
		restriction.Type = RestrictionTypeLimited
		restriction.MaxCopies = parseMaxCopies(cleanLabel)
	default:
		restriction.Type = RestrictionTypeLimited
		restriction.MaxCopies = parseMaxCopies(cleanLabel)
	}

	return restriction
}

func parseChooseOneOf(label string) int {
	if match := englishChooseOneOfRE.FindStringSubmatch(label); len(match) == 2 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	if match := japaneseChooseOneOfRE.FindStringSubmatch(label); len(match) == 2 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	return 0
}

func parseMaxCopies(label string) int {
	if match := englishUpToCopiesRE.FindStringSubmatch(label); len(match) == 2 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	if match := japaneseUpToCopiesRE.FindStringSubmatch(label); len(match) == 2 {
		n, _ := strconv.Atoi(match[1])
		return n
	}
	return 0
}

func parseFirstInt(label string) int {
	fields := strings.FieldsFunc(label, func(r rune) bool {
		return r < '0' || r > '9'
	})
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func extractCardNumbers(cell *goquery.Selection) []string {
	var cardNumbers []string
	cell.Find("a").Each(func(i int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if ok {
			if parsed, err := url.Parse(href); err == nil {
				if cardNo := parsed.Query().Get("cardno"); cardNo != "" {
					cardNumbers = append(cardNumbers, normalizeWhitespace(cardNo))
					return
				}
			}
		}
		text := normalizeWhitespace(s.Text())
		if text != "" {
			cardNumbers = append(cardNumbers, text)
		}
	})
	if len(cardNumbers) > 0 {
		return uniquePreserveOrder(cardNumbers)
	}
	text := normalizeWhitespace(selectionTextWithLineBreaks(cell))
	if text == "" {
		return nil
	}
	return []string{text}
}

func groupRestrictionsByName(restrictions []RestrictionGroup) map[string][]RestrictionGroup {
	grouped := make(map[string][]RestrictionGroup)
	for _, restriction := range restrictions {
		groupName, cleanedNotes := extractGroupName(restriction.Notes)
		restriction.Notes = cleanedNotes
		if groupName == "" {
			groupName = "unknown"
		}
		grouped[groupName] = append(grouped[groupName], restriction)
	}
	return grouped
}

func attachRestrictionGroupNotes(restrictions []RestrictionGroup) []RestrictionGroup {
	for i := range restrictions {
		restrictions[i].Notes = uniquePreserveOrder(restrictions[i].Notes)
	}
	return restrictions
}

func extractGroupName(notes []string) (string, []string) {
	var groupName string
	var cleaned []string
	for _, note := range notes {
		if strings.HasPrefix(note, "groupName=") {
			groupName = strings.TrimPrefix(note, "groupName=")
			continue
		}
		cleaned = append(cleaned, note)
	}
	return groupName, cleaned
}

func normalizeMarkerText(s string) string {
	s = strings.ReplaceAll(s, "(Updated!)", "")
	s = strings.ReplaceAll(s, "(New!)", "")
	return normalizeWhitespace(s)
}

func containsUpdatedMarker(s string) bool {
	return strings.Contains(s, "(Updated!)")
}

func containsNewMarker(s string) bool {
	return strings.Contains(s, "(New!)")
}

func selectionTextWithLineBreaks(sel *goquery.Selection) string {
	var b strings.Builder
	for _, node := range sel.Nodes {
		writeNodeText(&b, node)
	}
	return normalizeNewlines(b.String())
}

func writeNodeText(b *strings.Builder, node *html.Node) {
	switch node.Type {
	case html.TextNode:
		b.WriteString(node.Data)
	case html.ElementNode:
		if node.Data == "br" {
			b.WriteString("\n")
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			writeNodeText(b, child)
		}
		if node.Data == "p" || node.Data == "div" || node.Data == "tr" || node.Data == "td" {
			b.WriteString("\n")
		}
	default:
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			writeNodeText(b, child)
		}
	}
}

func splitCodes(raw string) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	parts := strings.Split(raw, ",")
	var codes []string
	for _, part := range parts {
		part = normalizeWhitespace(part)
		if part == "" {
			continue
		}
		codes = append(codes, part)
	}
	return uniquePreserveOrder(codes)
}

func uniquePreserveOrder(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, item := range in {
		item = normalizeWhitespace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
