package fetch

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func mustOpenDocument(t *testing.T, path string) *goquery.Document {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// matchesName reports whether name matches the group's canonical name or one of
// its aliases.
func (g TitleDeckGroup) matchesName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.EqualFold(g.CanonicalName, name) {
		return true
	}
	for _, alias := range g.Aliases {
		if strings.EqualFold(alias, name) {
			return true
		}
	}
	return false
}

func findGroup(t *testing.T, groups []TitleDeckGroup, side string, name string) TitleDeckGroup {
	t.Helper()
	for _, group := range groups {
		if group.Side == side && group.matchesName(name) {
			return group
		}
	}
	t.Fatalf("group not found for side=%q name=%q", side, name)
	return TitleDeckGroup{}
}

func assertContainsCode(t *testing.T, group TitleDeckGroup, code string) {
	t.Helper()
	if !slices.Contains(group.AllowedCodes, code) {
		t.Fatalf("expected %q in allowed codes for %q (%s); got %v", code, group.CanonicalName, group.Side, group.AllowedCodes)
	}
}

func loadEnglishDeckGroups(t *testing.T) []TitleDeckGroup {
	t.Helper()
	doc := mustOpenDocument(t, "mockws-en/deck_rules.html")
	groups, err := parseEnglishDeckConstruction(doc)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func loadEnglishDeckRules(t *testing.T) DeckRules {
	t.Helper()
	doc := mustOpenDocument(t, "mockws-en/deck_rules.html")
	rules, err := parseEnglishDeckRules(doc)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func loadJapaneseDeckGroups(t *testing.T) []TitleDeckGroup {
	t.Helper()
	doc := mustOpenDocument(t, "mockws/deck_rule.html")
	groups, err := parseJapaneseDeckConstruction(doc)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func loadJapaneseDeckRules(t *testing.T) DeckRules {
	t.Helper()
	doc := mustOpenDocument(t, "mockws/deck_rule.html")
	rules, err := parseJapaneseDeckRules(doc)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func findRestriction(t *testing.T, restrictions map[string][]RestrictionGroup, groupName string, restrictionType RestrictionType) RestrictionGroup {
	t.Helper()
	for _, restriction := range restrictions[groupName] {
		if restriction.Type == restrictionType {
			return restriction
		}
	}
	t.Fatalf("restriction not found for group=%q type=%q", groupName, restrictionType)
	return RestrictionGroup{}
}

func findFreeFloater(t *testing.T, floaters []FreeFloaterGroup, label string) FreeFloaterGroup {
	t.Helper()
	for _, floater := range floaters {
		if floater.Label == label {
			return floater
		}
	}
	t.Fatalf("free floater not found for label=%q", label)
	return FreeFloaterGroup{}
}

func findFreeFloaterCard(t *testing.T, floater FreeFloaterGroup, name string) RestrictionCard {
	t.Helper()
	for _, card := range floater.Cards {
		if card.Name == name {
			return card
		}
	}
	t.Fatalf("card %q not found in free floater %+v", name, floater)
	return RestrictionCard{}
}

func findRestrictionCard(t *testing.T, restriction RestrictionGroup, name string) RestrictionCard {
	t.Helper()
	for _, card := range restriction.Cards {
		if card.Name == name {
			return card
		}
	}
	t.Fatalf("card %q not found in restriction %+v", name, restriction)
	return RestrictionCard{}
}

func TestCanonicalizeTitleUsesSharedBaseAlias(t *testing.T) {
	canonical, aliases := canonicalizeTitle("BanG Dream!, BanG Dream! Girls Band Party!, BanG Dream! [MyGO!!!!!], BanG Dream! [Ave Mujica]")
	if canonical != "BanG Dream!" {
		t.Fatalf("unexpected canonical name: %q", canonical)
	}
	if slices.Contains(aliases, canonical) {
		t.Fatalf("canonical name should not be repeated in aliases: %v", aliases)
	}
	for _, alias := range []string{
		"BanG Dream! Girls Band Party!",
		"BanG Dream! [MyGO!!!!!]",
		"BanG Dream! [Ave Mujica]",
	} {
		if !slices.Contains(aliases, alias) {
			t.Fatalf("missing alias %q in %v", alias, aliases)
		}
	}
}

func TestCanonicalizeTitleUsesSharedBaseAliasRegardlessOfOrder(t *testing.T) {
	canonical, aliases := canonicalizeTitle("BanG Dream! [Ave Mujica], BanG Dream!, BanG Dream! Girls Band Party!")
	if canonical != "BanG Dream!" {
		t.Fatalf("unexpected canonical name: %q", canonical)
	}
	if slices.Contains(aliases, canonical) {
		t.Fatalf("canonical name should not be repeated in aliases: %v", aliases)
	}
}

func TestCanonicalizeTitleSupportsFutureBandoriAliasVariants(t *testing.T) {
	canonical, aliases := canonicalizeTitle("BanG Dream!, BanG Dream! [Yume∞Mita], BanG Dream! Girls Band Party!")
	if canonical != "BanG Dream!" {
		t.Fatalf("unexpected canonical name: %q", canonical)
	}
	if slices.Contains(aliases, canonical) {
		t.Fatalf("canonical name should not be repeated in aliases: %v", aliases)
	}
	for _, alias := range []string{
		"BanG Dream! [Yume∞Mita]",
		"BanG Dream! Girls Band Party!",
	} {
		if !slices.Contains(aliases, alias) {
			t.Fatalf("missing alias %q in %v", alias, aliases)
		}
	}
}

func TestCanonicalizeTitleFallsBackToRawCombinedTitle(t *testing.T) {
	canonical, aliases := canonicalizeTitle("Date A Live, Date A Bullet")
	if canonical != "Date A Live, Date A Bullet" {
		t.Fatalf("unexpected canonical name: %q", canonical)
	}
	if len(aliases) != 2 {
		t.Fatalf("unexpected aliases: %v", aliases)
	}
}

func TestParseEnglishDeckConstructionBandoriAliases(t *testing.T) {
	groups := loadEnglishDeckGroups(t)
	bandori := findGroup(t, groups, "W", "BanG Dream! [Ave Mujica]")
	if bandori.CanonicalName != "BanG Dream!" {
		t.Fatalf("unexpected canonical name: %q", bandori.CanonicalName)
	}
	assertContainsCode(t, bandori, "BD")
	if slices.Contains(bandori.Aliases, bandori.CanonicalName) {
		t.Fatalf("canonical name should not be in aliases: %v", bandori.Aliases)
	}
	for _, alias := range []string{
		"BanG Dream! Girls Band Party!",
		"BanG Dream! [MyGO!!!!!]",
		"BanG Dream! [Ave Mujica]",
	} {
		if !slices.Contains(bandori.Aliases, alias) {
			t.Fatalf("missing alias %q in %v", alias, bandori.Aliases)
		}
	}
}

func TestParseEnglishDeckConstructionDengekiIsSideSpecific(t *testing.T) {
	groups := loadEnglishDeckGroups(t)
	dengekiW := findGroup(t, groups, "W", "Dengeki Bunko")
	dengekiS := findGroup(t, groups, "S", "Dengeki Bunko")
	if slices.Equal(dengekiW.AllowedCodes, dengekiS.AllowedCodes) {
		t.Fatalf("expected side-specific Dengeki lists, got equal lists")
	}
}

func TestParseEnglishDeckConstructionSplitsMultiCodeRows(t *testing.T) {
	groups := loadEnglishDeckGroups(t)
	loveLiveSunshine := findGroup(t, groups, "W", "Love Live! Sunshine!!")
	for _, code := range []string{"SIS", "LSF", "LSS"} {
		assertContainsCode(t, loveLiveSunshine, code)
	}
}

func TestParseEnglishDeckConstructionPreservesSharedCodesAcrossTitles(t *testing.T) {
	groups := loadEnglishDeckGroups(t)
	dengekiS := findGroup(t, groups, "S", "Dengeki Bunko")
	sao := findGroup(t, groups, "S", "Sword Art Online")
	assertContainsCode(t, sao, "Gso")
	assertContainsCode(t, dengekiS, "Gso")
}

func TestParseEnglishDeckConstructionReturnsSideAndCodes(t *testing.T) {
	groups := loadEnglishDeckGroups(t)
	for _, group := range groups {
		if group.Side == "" {
			t.Fatalf("group %q has empty side", group.CanonicalName)
		}
		if len(group.AllowedCodes) == 0 {
			t.Fatalf("group %q has empty code list", group.CanonicalName)
		}
	}
}

func TestParseEnglishDeckRulesIncludesUnrestrictedCards(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	unrestricted := findFreeFloater(t, rules.FreeFloaters, "You may use the following cards in any format, unless stated otherwise.")
	card := findFreeFloaterCard(t, unrestricted, "Lunar New Year 2024, Shiyoko")
	if !slices.Contains(card.CardNumbers, "CGS/WS01-PE32") {
		t.Fatalf("unexpected unrestricted card numbers: %v", card.CardNumbers)
	}
}

func TestParseEnglishDeckRulesParsesLimitedRestrictions(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	limited := findRestriction(t, rules.RestrictionsByGroup, "hololive Production", RestrictionTypeLimited)
	if limited.MaxCopies != 2 {
		t.Fatalf("unexpected max copies: %d", limited.MaxCopies)
	}
	card := findRestrictionCard(t, limited, "A Step Towards The Future, Gawr Gura")
	if !slices.Contains(card.CardNumbers, "HOL/W104-E113") {
		t.Fatalf("unexpected card numbers: %v", card.CardNumbers)
	}
}

func TestParseEnglishDeckRulesParsesRestrictedRestrictions(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	restricted := findRestriction(t, rules.RestrictionsByGroup, "Avatar: The Last Airbender", RestrictionTypeRestricted)
	card := findRestrictionCard(t, restricted, "Sokka: Offering Different Perspectives")
	if !slices.Contains(card.CardNumbers, "ATLA/WX04-082") {
		t.Fatalf("unexpected card numbers: %v", card.CardNumbers)
	}
}

func TestParseEnglishDeckRulesParsesChoiceRestrictions(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	choice := findRestriction(t, rules.RestrictionsByGroup, "The Quintessential Quintuplets", RestrictionTypeChoiceRestriction)
	if choice.ChooseOneOf != 5 {
		t.Fatalf("unexpected choose-one-of value: %d", choice.ChooseOneOf)
	}
	if choice.MaxCopies != 4 {
		t.Fatalf("unexpected max copies for choice restriction: %d", choice.MaxCopies)
	}
	card := findRestrictionCard(t, choice, "Daily Routine, Nino Nakano")
	if !slices.Contains(card.CardNumbers, "5HY/W90-E053") {
		t.Fatalf("unexpected card numbers: %v", card.CardNumbers)
	}
}

func TestParseEnglishDeckRulesParsesAnyCombinationRestrictions(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	combination := findRestriction(t, rules.RestrictionsByGroup, "Fate", RestrictionTypeAnyCombination)
	if combination.MaxCopies != 4 {
		t.Fatalf("unexpected max copies: %d", combination.MaxCopies)
	}
	if len(combination.Cards) != 2 {
		t.Fatalf("unexpected any-combination card count: %d", len(combination.Cards))
	}
}

func TestParseEnglishDeckRulesPreservesRestrictionNotes(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	choice := findRestriction(t, rules.RestrictionsByGroup, "Batman Ninja", RestrictionTypeChoiceRestriction)
	if len(choice.Notes) == 0 {
		t.Fatalf("expected note on Batman Ninja restriction")
	}
}

func TestParseEnglishDeckRulesStripsUpdateMarkersAndTracksThem(t *testing.T) {
	rules := loadEnglishDeckRules(t)
	choice := findRestriction(t, rules.RestrictionsByGroup, "The Quintessential Quintuplets", RestrictionTypeChoiceRestriction)
	if choice.Label != "Choice Restriction (Choose 1 of 5)" {
		t.Fatalf("unexpected normalized label: %q", choice.Label)
	}
	if !choice.Updated {
		t.Fatalf("expected updated flag on choice restriction")
	}

	limited := findRestriction(t, rules.RestrictionsByGroup, "Rascal Does Not Dream", RestrictionTypeLimited)
	if limited.Label != "Limited (Up to 2 copies)" {
		t.Fatalf("unexpected normalized limited label: %q", limited.Label)
	}
	if !limited.Updated {
		t.Fatalf("expected updated flag on limited restriction")
	}
	card := findRestrictionCard(t, limited, "The Year I Spent With You, Mai Sakurajima")
	if !card.IsNew {
		t.Fatalf("expected new flag on card")
	}
}

func TestParseJapaneseDeckConstructionBandoriCodes(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)

	bandori := findGroup(t, groups, "W", "BanG Dream!")
	for _, code := range []string{"BD", "BDY"} {
		assertContainsCode(t, bandori, code)
	}
}

func TestParseJapaneseDeckConstructionDengekiUsesWeissSubset(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)
	dengekiW := findGroup(t, groups, "W", "電撃文庫")
	assertContainsCode(t, dengekiW, "Gas")
	assertContainsCode(t, dengekiW, "Gsr")
	if slices.Contains(dengekiW.AllowedCodes, "G86") {
		t.Fatalf("unexpected Schwarz-only code G86 in Weiss Dengeki list: %v", dengekiW.AllowedCodes)
	}
}

func TestParseJapaneseDeckConstructionDengekiUsesSchwarzSubset(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)
	dengekiS := findGroup(t, groups, "S", "電撃文庫")
	assertContainsCode(t, dengekiS, "G86")
	assertContainsCode(t, dengekiS, "Gso")
	if len(dengekiS.Notes) == 0 {
		t.Fatalf("expected notes for Japanese Dengeki row")
	}
}

func TestParseJapaneseDeckConstructionKeepsDualSideTitlesSeparate(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)

	shiyokoW := findGroup(t, groups, "W", "カードゲームしよ子")
	shiyokoS := findGroup(t, groups, "S", "カードゲームしよ子")
	assertContainsCode(t, shiyokoW, "CGS")
	assertContainsCode(t, shiyokoS, "SI")
}

func TestParseJapaneseDeckConstructionPreservesSharedCodesAcrossTitles(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)

	dengekiS := findGroup(t, groups, "S", "電撃文庫")
	sao := findGroup(t, groups, "S", "ソードアート・オンライン")
	assertContainsCode(t, sao, "Gso")
	assertContainsCode(t, dengekiS, "Gso")
}

func TestParseJapaneseDeckConstructionReturnsSideAndCodes(t *testing.T) {
	groups := loadJapaneseDeckGroups(t)
	for _, group := range groups {
		if group.Side == "" {
			t.Fatalf("group %q has empty side", group.CanonicalName)
		}
		if len(group.AllowedCodes) == 0 {
			t.Fatalf("group %q has empty code list", group.CanonicalName)
		}
	}
}

func TestParseJapaneseDeckRulesIncludesUnrestrictedCards(t *testing.T) {
	rules := loadJapaneseDeckRules(t)
	unrestricted := findFreeFloater(t, rules.FreeFloaters, "すべてのタイトルで使用可能")
	card := findFreeFloaterCard(t, unrestricted, "カルチャージャパンのアイドル みらい")
	if !slices.Contains(card.CardNumbers, "CJ/MIR-001") {
		t.Fatalf("unexpected unrestricted card numbers: %v", card.CardNumbers)
	}
}

func TestParseJapaneseDeckRulesParsesRestrictedRestrictions(t *testing.T) {
	rules := loadJapaneseDeckRules(t)
	restricted := findRestriction(t, rules.RestrictionsByGroup, "ソードアート・オンライン", RestrictionTypeRestricted)
	card := findRestrictionCard(t, restricted, "真っ直ぐな道 アリス")
	if !slices.Contains(card.CardNumbers, "SAO/S100-011") {
		t.Fatalf("unexpected restricted card numbers: %v", card.CardNumbers)
	}
}

func TestParseJapaneseDeckRulesParsesLimitedRestrictions(t *testing.T) {
	rules := loadJapaneseDeckRules(t)
	limited := findRestriction(t, rules.RestrictionsByGroup, "デート・ア・ライブ", RestrictionTypeLimited)
	if limited.MaxCopies != 2 {
		t.Fatalf("unexpected max copies: %d", limited.MaxCopies)
	}
	card := findRestrictionCard(t, limited, "“最悪の精霊”狂三")
	if !slices.Contains(card.CardNumbers, "DAL/W79-053") {
		t.Fatalf("unexpected limited card numbers: %v", card.CardNumbers)
	}
}

func TestParseJapaneseDeckRulesParsesChoiceRestrictions(t *testing.T) {
	rules := loadJapaneseDeckRules(t)
	choice := findRestriction(t, rules.RestrictionsByGroup, "ラブライブ！スーパースター!!", RestrictionTypeChoiceRestriction)
	if choice.ChooseOneOf != 2 {
		t.Fatalf("unexpected choose-one-of value: %d", choice.ChooseOneOf)
	}
	if choice.MaxCopies != 4 {
		t.Fatalf("unexpected max copies for choice restriction: %d", choice.MaxCopies)
	}
	if len(choice.Cards) != 2 {
		t.Fatalf("unexpected choice restriction card count: %d", len(choice.Cards))
	}
}
