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

package fetch

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/language"
)

// Card info to export
type Card struct {
	// CardNumber is the full card number/code used to identify each card.
	// It typically consists of the SetID, Side, Release, ReleasePackID, and ID,
	// though the format is different in some situations.
	CardNumber string `json:"cardNumber"`
	// SetID is the alphanumeric string found at the beginning of card numbers,
	// before the "/"".
	SetID string `json:"setId"`
	// SetName is the official name of the set/IP.
	// It is currently unset by the scraper and is reserved for when the websites
	// expose one or the scraper is updated to find it.
	SetName string `json:"setName"`
	// ExpansionName is the normalized product/expansion title shown on card pages
	// (eg. "Love Live! Vol.2"). This may differ from the product page title.
	ExpansionName string `json:"expansionName"`
	// ExpansionSlug is the official product slug for normal releases when available.
	// Promo cards use a best-effort code from promo metadata, with ReleasePackID as
	// a fallback when the listing doesn't expose one.
	ExpansionSlug string `json:"expansionSlug"`
	// ExpansionProductDisplayName is the product page title for normal releases or
	// the specific promo group/distribution name from the promo listing.
	ExpansionProductDisplayName string `json:"expansionProductDisplayName"`
	// ExpansionProductURL is the product page URL when the card page links to one.
	ExpansionProductURL string `json:"expansionProductURL,omitempty"`
	// ExpansionSourceType indicates whether the richer expansion metadata came from
	// a linked product page or an official promo listing.
	ExpansionSourceType ExpansionSourceType `json:"expansionSourceType"`
	// Sides contains the card's side ("W" for Weiss, "S" for Schwarz).
	// Some cards are dual-sided (eg. Gso/WS02-124SP and Gso/WS02-E124SP).
	Sides []Side `json:"sides,omitempty"`
	// Release typically consists of the card's side, followed by a number
	// (the release pack ID) indicating which consecutive release for the relative
	// side the release is.
	// For example, "W64" would mean the 64th set of the Weiss side.
	// There are certain situations that don't follow the aforementioned format,
	// such as with promo cards (eg. BSF2024) or special sets (eg. EN-W03).
	Release string `json:"release"`
	// ReleasePackID indicates which consecutive release for the relative
	// side the release is.
	// For example, "W64" would mean the 64th set of the Weiss side.
	// For cards with non-standard release codes, a best-effort/most sensible
	// ID is chosen (eg. 2021 from BSL2021). This may be empty if there's
	// no sensible ID to choose (eg. from TCPR-P01).
	ReleasePackID string `json:"releasePackId"`
	// ID of the card within the set+release. This is usually the last part
	// of the card number (after the -).
	ID string `json:"id"`
	// Language the card is printed in.
	Language string `json:"language"`

	// Type can be either "CH" for character, "EV" for event, or "CX" for climax.
	Type CardType `json:"type"`

	// Name of the card.
	Name string `json:"name"`
	// Color of the card. Should be either "BLUE", "GREEN", "RED", or "YELLOW".
	// ...Except for the two purple cards (むらさきパプリス(PY/S38-125) and むらさきぷよ(PY/S38-120)).
	Color CardColor `json:"color"`
	// Stock cost to play the card.
	Cost *int `json:"cost,omitempty"`
	// Level required in order to play the card.
	Level *int `json:"level,omitempty"`
	// Power indicates the card's battle strength. Only valid for Character cards.
	Power *int `json:"power,omitempty"`
	// Soul indicates how many soul points the card has. Only valid for Character cards.
	Soul *int `json:"soul,omitempty"`
	// Text describing the card's abilities.
	Text []string `json:"text"`
	// Traits indicating the attributes the card has. These are often referenced in card text.
	Traits []string `json:"traits,omitempty"`
	// Triggers that the card has and are activated during trigger checks.
	Triggers []Trigger `json:"triggers,omitempty"`
	// ParseFailures captures non-fatal parsing issues where the remaining card data
	// still looked trustworthy enough to keep.
	ParseFailures []string `json:"parseFailures,omitempty"`

	FlavorText string      `json:"flavorText,omitempty"`
	ImageURL   string      `json:"imageURL"`
	Image      image.Image `json:"-"`
	Rarity     string      `json:"rarity"`
}

var (
	standardCardSuffixRE = regexp.MustCompile(`(?P<setID>[a-zA-Z0-9]+)/(?P<release>[a-zA-Z0-9-]+)[-_](?P<id>[a-zA-Z0-9_]+\+?)$`)

	standardReleaseRE = regexp.MustCompile(`(?P<code>[a-zA-Z-]+)(?P<packID>[0-9]+)`)
)

var suffix = []string{
	"SP",
	"S",
	"R",
}

var baseRarity = []string{
	"C",
	"CC",
	"CR",
	"FR",
	"MR",
	"PR",
	"PS",
	"R",
	"RE",
	"RR",
	"RR+",
	"TD",
	"U",
	"AR",
}

var triggersMap = map[string]Trigger{
	"soul":      TriggerSoul,
	"salvage":   TriggerComeback,
	"comeback":  TriggerComeback,
	"draw":      TriggerDraw,
	"focus":     TriggerFocus,
	"stock":     TriggerPool,
	"pool":      TriggerPool,
	"treasure":  TriggerTreasure,
	"shot":      TriggerShot,
	"bounce":    TriggerReturn,
	"return":    TriggerReturn,
	"gate":      TriggerGate,
	"standby":   TriggerStandby,
	"chance":    TriggerChance,
	"choice":    TriggerChoice,
	"discovery": TriggerDiscovery,
}

var jpTextColorMap = map[string]CardColor{
	"青": CardColorBlue,
	"緑": CardColorGreen,
	"赤": CardColorRed,
	"黄": CardColorYellow,
	"紫": CardColorPurple,
}

func parseNumericStat(st string) *int {
	st = strings.TrimSpace(st)
	if st == "" || strings.Contains(st, "-") {
		return nil
	}

	n, err := strconv.Atoi(st)
	if err != nil {
		return nil
	}

	return &n
}

func parseSides(sideNode *goquery.Selection) []Side {
	if sideNode == nil {
		return nil
	}

	found := map[Side]bool{}
	sideNode.Find("img").Each(func(i int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if !ok {
			return
		}
		_, sideName := path.Split(src)
		switch strings.ToUpper(strings.Split(sideName, ".")[0]) {
		case "W":
			found[SideWeiss] = true
		case "S":
			found[SideSchwarz] = true
		}
	})

	var sides []Side
	if found[SideWeiss] {
		sides = append(sides, SideWeiss)
	}
	if found[SideSchwarz] {
		sides = append(sides, SideSchwarz)
	}
	return sides
}

// extractData extract data to card
func extractData(config siteConfig, mainHTML *goquery.Selection) Card {
	switch config.languageCode {
	case language.English:
		return extractDataEn(config, mainHTML)
	case language.Japanese:
		return extractDataJp(config, mainHTML)
	default:
		slog.Error(fmt.Sprintf("Unsupported site: %q", config.languageCode))
		return Card{}
	}
}

func extractDataEn(config siteConfig, mainHTML *goquery.Selection) Card {
	txtArea := mainHTML.Find(".p-cards__detail-textarea").Last()
	cardNumber := txtArea.Find(".number").First().Last().Text()
	defer func() {
		if err := recover(); err != nil {
			slog.With("cardnumber", cardNumber).Error(fmt.Sprintf("Panic during card extraction=%v", err))
		}
	}()

	cardNumber = sanitizeCardNumber(cardNumber)
	slog.Debug(fmt.Sprintf("Start card: %s", cardNumber))

	setID, release, releasePackID, cardID := parseCardNumber(cardNumber)

	cardName := txtArea.Find(".ttl").First().Text()
	imageCardURL, _ := mainHTML.Find("div.image img").Attr("src")

	info := make(map[string]string)
	cardFailures := make([]string, 0)
	mainHTML.Find("dl").Each(func(i int, s *goquery.Selection) {
		dt := strings.TrimSpace(s.Find("dt").First().Text())
		dd := s.Find("dd").First()
		ddText := strings.TrimSpace(dd.Text())
		switch dt {
		case "Card Type":
			switch ddText {
			case "Event":
				info["type"] = string(CardTypeEvent)
			case "Character":
				info["type"] = string(CardTypeCharacter)
			case "Climax":
				info["type"] = string(CardTypeClimax)
			}
		case "Color":
			if u, ok := dd.Find("img").First().Attr("src"); ok {
				_, colorName := path.Split(u)
				info["color"] = strings.ToUpper(strings.Split(colorName, ".")[0])
			} else if strings.HasPrefix(ddText, "[[") && strings.HasSuffix(ddText, "]]") {
				// Handle case where color is in text format like [[yellow.gif]]
				colorName := strings.TrimSuffix(strings.TrimPrefix(ddText, "[["), "]]")
				info["color"] = strings.ToUpper(strings.Split(colorName, ".")[0])
			} else {
				slog.With("cardnumber", cardNumber).Error("Failed to get color", "ddText", ddText)
			}
		case "Cost":
			info["cost"] = ddText
		case "Expansion":
			info["expansion"] = ddText
		case "Level":
			info["level"] = ddText
		case "Power":
			info["power"] = ddText
		case "Rarity":
			info["rarity"] = ddText
		case "Side":
			sides := parseSides(dd)
			if len(sides) == 0 {
				slog.With("cardnumber", cardNumber).Error("Failed to get side")
				return
			}
			info["sides"] = strings.Join(sidesToStrings(sides), " ")
		case "Soul":
			info["soul"] = strconv.Itoa(dd.Children().Length())
		case "Traits":
			info["specialAttribute"] = ddText
		case "Trigger":
			triggers, failures := parseTriggers(dd, cardNumber)
			if len(triggers) != 0 {
				info["trigger"] = strings.Join(triggersToStrings(triggers), " ")
			}
			cardFailures = append(cardFailures, failures...)
		default:
			slog.With("cardnumber", cardNumber).Error(fmt.Sprintf("Unknown detail: %v", dt))
		}
	})

	// Flavor text
	flvr := strings.TrimSpace(txtArea.Find(".p-cards__detail-serif").Text())
	if flvr != "" && flvr != "-" && flvr != "―" {
		info["flavourText"] = flvr
	}

	ability, err := extractAbilities(mainHTML.Find(".p-cards__detail p").Last())
	if err != nil {
		slog.With("cardnumber", cardNumber).Error(fmt.Sprintf("Failed to get ability node: %v", err))
	}

	card := Card{
		CardNumber: cardNumber,
		SetID:      setID,
		// TODO: Figure out how to get EN set name. It's no longer on the card details page
		// SetName:     setName,
		ExpansionName: info["expansion"],
		Sides:         parseSideFields(info["sides"]),
		Release:       release,
		ReleasePackID: releasePackID,
		ID:            cardID,
		Language:      language.English.String(),
		Type:          CardType(info["type"]),
		Name:          cardName,
		Level:         parseNumericStat(info["level"]),
		Cost:          parseNumericStat(info["cost"]),
		FlavorText:    info["flavourText"],
		Color:         CardColor(info["color"]),
		Power:         parseNumericStat(info["power"]),
		Rarity:        info["rarity"],
		Text:          ability,
		ParseFailures: cardFailures,
	}
	if fullURL, err := joinPath(config.baseURL, imageCardURL); err == nil {
		card.ImageURL = fullURL.String()
	} else {
		slog.With("cardnumber", cardNumber).Error(fmt.Sprintf("Couldn't form full image URL: %v", err))
		card.ImageURL = imageCardURL
	}
	if info["specialAttribute"] != "" {
		card.Traits = strings.Split(info["specialAttribute"], "・")
	}
	card.Triggers = parseTriggerFields(info["trigger"])
	if card.Type == CardTypeCharacter {
		card.Soul = parseNumericStat(info["soul"])
	}
	applyExpansionMetadata(&card, extractExpansionMetadata(config, mainHTML))
	return card
}

func extractDataJp(config siteConfig, mainHTML *goquery.Selection) Card {
	rawCardNumber := mainHTML.Find("h4 span").Last().Text()
	defer func() {
		if err := recover(); err != nil {
			slog.With("cardnumber", rawCardNumber).Error(fmt.Sprintf("Panic during card extraction=%v", err))
		}
	}()

	cardNumber := sanitizeCardNumber(rawCardNumber)
	slog.Debug(fmt.Sprintf("Start card: %s", rawCardNumber))

	setID, release, releasePackID, cardID := parseCardNumber(cardNumber)

	expansionName := strings.TrimSpace(strings.Split(mainHTML.Find("h4").Text(), ") -")[1])
	imageCardURL, _ := mainHTML.Find("a img").Attr("src")

	ability, err := extractAbilities(mainHTML.Find("span").Last())
	if err != nil {
		slog.With("cardnumber", rawCardNumber).Error(fmt.Sprintf("Failed to get ability node: %v", err))
	}

	infos := make(map[string]string)
	cardFailures := make([]string, 0)
	mainHTML.Find(".unit").Each(func(i int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		switch {
		// Color
		case strings.HasPrefix(txt, "色："):
			colorText := strings.TrimSpace(strings.TrimPrefix(txt, "色："))
			if color, ok := jpTextColorMap[colorText]; ok {
				infos["color"] = string(color)
			} else if colorText != "" && colorText != "-" && colorText != "－" {
				infos["color"] = strings.ToUpper(colorText)
			} else if colorSrc, ok := s.Children().Attr("src"); ok {
				_, colorName := path.Split(colorSrc)
				infos["color"] = strings.ToUpper(strings.Split(colorName, ".")[0])
			} else {
				if colorText != "" {
					infos["color"] = strings.ToUpper(colorText)
				}
			}
			// Card type
		case strings.HasPrefix(txt, "種類："):
			cType := strings.TrimSpace(strings.TrimPrefix(txt, "種類："))

			switch cType {
			case "イベント":
				infos["type"] = string(CardTypeEvent)
			case "キャラ":
				infos["type"] = string(CardTypeCharacter)
			case "クライマックス":
				infos["type"] = string(CardTypeClimax)
			}
			// Cost
		case strings.HasPrefix(txt, "コスト："):
			cost := strings.TrimSpace(strings.TrimPrefix(txt, "コスト："))
			infos["cost"] = cost
			// Flavor text
		case strings.HasPrefix(txt, "フレーバー："):
			flvr := strings.TrimSpace(strings.TrimPrefix(txt, "フレーバー："))
			infos["flavourText"] = flvr
			// Level
		case strings.HasPrefix(txt, "レベル："):
			lvl := strings.TrimSpace(strings.TrimPrefix(txt, "レベル："))
			infos["level"] = lvl
			// Power
		case strings.HasPrefix(txt, "パワー："):
			pwr := strings.TrimSpace(strings.TrimPrefix(txt, "パワー："))
			infos["power"] = pwr
			// Rarity
		case strings.HasPrefix(txt, "レアリティ："):
			rarity := strings.TrimSpace(strings.TrimPrefix(txt, "レアリティ："))
			infos["rarity"] = rarity
			// Side
		case strings.HasPrefix(txt, "サイド："):
			sides := parseSides(s)
			if len(sides) == 0 {
				break
			}
			infos["sides"] = strings.Join(sidesToStrings(sides), " ")
			// Soul
		case strings.HasPrefix(txt, "ソウル："):
			infos["soul"] = strconv.Itoa(s.Children().Length())
			// Trigger
		case strings.HasPrefix(txt, "トリガー："):
			triggers, failures := parseTriggers(s, rawCardNumber)
			if len(triggers) != 0 {
				infos["trigger"] = strings.Join(triggersToStrings(triggers), " ")
			}
			cardFailures = append(cardFailures, failures...)
			// Trait
		case strings.HasPrefix(txt, "特徴："):
			var res bytes.Buffer
			s.Children().Each(func(i int, ss *goquery.Selection) {
				res.WriteString(strings.TrimSpace(ss.Text()))
			})
			if strings.Contains(res.String(), "-") {
				infos["specialAttribute"] = ""
			} else {
				infos["specialAttribute"] = strings.TrimSpace(res.String())
			}
		default:
			slog.With("cardnumber", rawCardNumber).Error(fmt.Sprintf("Unknown detail: %q", txt))
		}
	})

	card := Card{
		CardNumber:    cardNumber,
		SetID:         setID,
		ExpansionName: expansionName,
		Sides:         parseSideFields(infos["sides"]),
		Release:       release,
		ReleasePackID: releasePackID,
		ID:            cardID,
		Language:      language.Japanese.String(),
		Type:          CardType(infos["type"]),
		Name:          strings.TrimSpace(mainHTML.Find("h4 span").First().Text()),
		Level:         parseNumericStat(infos["level"]),
		FlavorText:    infos["flavourText"],
		Color:         CardColor(infos["color"]),
		Power:         parseNumericStat(infos["power"]),
		Cost:          parseNumericStat(infos["cost"]),
		Rarity:        infos["rarity"],
		Text:          ability,
		ParseFailures: cardFailures,
	}
	if fullURL, err := joinPath(config.baseURL, imageCardURL); err == nil {
		card.ImageURL = fullURL.String()
	} else {
		slog.With("cardnumber", rawCardNumber).Error(fmt.Sprintf("Couldn't form full image URL: %v", err))
		card.ImageURL = imageCardURL
	}
	if infos["specialAttribute"] != "" {
		card.Traits = strings.Split(infos["specialAttribute"], "・")
	}
	card.Triggers = parseTriggerFields(infos["trigger"])
	if card.Type == CardTypeCharacter {
		card.Soul = parseNumericStat(infos["soul"])
	}
	applyExpansionMetadata(&card, extractExpansionMetadata(config, mainHTML))
	return card
}

func parseSideFields(s string) []Side {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}

	sides := make([]Side, 0, len(fields))
	for _, field := range fields {
		sides = append(sides, Side(field))
	}
	return sides
}

func sidesToStrings(sides []Side) []string {
	if len(sides) == 0 {
		return nil
	}

	out := make([]string, 0, len(sides))
	for _, side := range sides {
		out = append(out, string(side))
	}
	return out
}

func parseTriggerFields(s string) []Trigger {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}

	triggers := make([]Trigger, 0, len(fields))
	for _, field := range fields {
		triggers = append(triggers, Trigger(field))
	}
	return triggers
}

func triggersToStrings(triggers []Trigger) []string {
	if len(triggers) == 0 {
		return nil
	}

	out := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		out = append(out, string(trigger))
	}
	return out
}

func parseTriggers(node *goquery.Selection, cardNumber string) ([]Trigger, []string) {
	if node == nil {
		return nil, nil
	}

	triggers := make([]Trigger, 0, node.Children().Length())
	failures := make([]string, 0)
	node.Children().Each(func(i int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if !ok {
			return
		}

		_, triggerFile := path.Split(src)
		triggerName := strings.Split(triggerFile, ".")[0]
		trigger, ok := triggersMap[triggerName]
		if !ok {
			failure := fmt.Sprintf("unknown trigger icon: %s", triggerName)
			slog.With("cardnumber", cardNumber, "trigger", triggerName).Warn("Non-fatal trigger parse failure")
			failures = append(failures, failure)
			return
		}

		triggers = append(triggers, trigger)
	})
	return triggers, failures
}

func extractAbilities(abilityNode *goquery.Selection) ([]string, error) {
	var ability []string
	abilityNode.Find("img").Each(func(i int, s *goquery.Selection) {
		url, has := s.Attr("src")
		if has {
			_, _imgPlaceHolder := path.Split(url)
			_imgPlaceHolder = strings.Split(_imgPlaceHolder, ".")[0]
			if trigger, ok := triggersMap[_imgPlaceHolder]; ok {
				t := fmt.Sprintf("[%s]", string(trigger))
				s.ReplaceWithHtml(t)
			}
		}
	})
	abilityNodeHtml, err := abilityNode.Html()
	if err != nil {
		err = fmt.Errorf("failed to get ability node: %v", err)
	}
	for _, line := range strings.Split(abilityNodeHtml, "<br/>") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ability = append(ability, html.UnescapeString(line))
	}
	return ability, err
}

func sanitizeCardNumber(cn string) string {
	// The website sometimes shows "%2B" instead of + for some cards (eg. SSP+ rarity).
	cn = strings.ReplaceAll(cn, "%2B", "+")

	// Replace underscores with appropriate characters before parsing
	// First underscore after release code becomes hyphen, rest become spaces
	parts := strings.Split(cn, "/")
	if len(parts) > 1 {
		beforeSlash := parts[0]
		afterSlash := parts[1]

		// Find the first underscore after the release code
		releaseEnd := strings.IndexAny(afterSlash, "-_")
		if releaseEnd != -1 && strings.Contains(afterSlash, "_") {
			release := afterSlash[:releaseEnd]
			rest := afterSlash[releaseEnd:]
			// Replace first underscore with hyphen, rest with spaces
			rest = strings.Replace(rest, "_", "-", 1)
			rest = strings.ReplaceAll(rest, "_", " ")
			afterSlash = release + rest
		}

		cn = beforeSlash + "/" + afterSlash
	}

	// The website sometimes puts a + in the displayed card number (eg. RWBY/BRO2021-01+PR) even though it shouldn't be there.
	plusCnt := strings.Count(cn, "+")
	if plusCnt > 0 {
		repCnt := plusCnt
		if strings.LastIndex(cn, "+") == len(cn)-1 {
			repCnt--
		}
		cn = strings.Replace(cn, "+", " ", repCnt)
	}
	return cn
}

func parseCardNumber(cn string) (setID, release, releasePackID, id string) {
	if matches := standardCardSuffixRE.FindStringSubmatch(cn); matches != nil {
		setID = matches[1]
		release = matches[2]
		id = matches[3]
		if relMatches := standardReleaseRE.FindStringSubmatch(release); relMatches != nil {
			releasePackID = relMatches[2]
			return
		}
		releasePackID = ""
		return
	}

	if strings.Contains(cn, "/") {
		setID = strings.Split(cn, "/")[0]
		setInfo := strings.Split(strings.Split(cn, "/")[1], "-")
		if len(setInfo) > 1 {
			release = setInfo[0]
			id = setInfo[1]

			if relMatches := standardReleaseRE.FindStringSubmatch(release); relMatches != nil {
				releasePackID = relMatches[2]
				return
			}
			releasePackID = ""
		}
		return
	} else {
		slog.With("cardnumber", cn).Error(fmt.Sprintf("Can't get set info from: %s", cn))
	}
	return
}

// IsbaseRarity check if a card is a C / U / R / RR
func IsbaseRarity(card Card) bool {
	for _, rarity := range baseRarity {
		if rarity == card.Rarity && isTrullyNotFoil(card) {
			return true
		}
	}
	return false
}

func isTrullyNotFoil(card Card) bool {
	for _, _suffix := range suffix {
		if strings.HasSuffix(card.ID, _suffix) {
			return false
		}
	}
	return true
}
