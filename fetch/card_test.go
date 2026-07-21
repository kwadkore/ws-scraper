package fetch

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func intPtr(v int) *int {
	return &v
}

func TestCardFromJapaneseAPIItem(t *testing.T) {
	item := japaneseCardItem{
		ID:          2331,
		CardNumber:  "DC/W01-002",
		TitleNumber: "DC",
		CardName:    "朝倉 音姫",
		CardKind:    "2",
		Color:       "[[yellow.gif]]",
		Level:       "1",
		Cost:        "1",
		Power:       "6000",
		Soul:        "[[soul.gif]]",
		CardTrigger: "[[soul.gif]][[gate.gif]]",
		Text:        "【自】 アンコール<br />【起】 集中",
		Flavor:      "お昼まだでしょ？",
		Picture:     "d/dc_w01/dc_w01_002.png",
		Expansion:   1,
		Rare:        "RR",
		Feature1:    "魔法",
		Feature2:    "生徒会",
		Feature3:    "-",
		Side:        "-1",
	}

	card := cardFromJapaneseAPIItem(siteConfigs[Japanese], item, "D.C. D.C.II", nil)
	want := Card{
		CardNumber:    "DC/W01-002",
		SetID:         "DC",
		ExpansionName: "D.C. D.C.II",
		Sides:         []Side{SideWeiss},
		Release:       "W01",
		ReleasePackID: "01",
		ID:            "002",
		Language:      "ja",
		Type:          CardTypeCharacter,
		Name:          "朝倉 音姫",
		Color:         CardColorYellow,
		Level:         intPtr(1),
		Cost:          intPtr(1),
		Power:         intPtr(6000),
		Soul:          intPtr(1),
		FlavorText:    "お昼まだでしょ？",
		Rarity:        "RR",
		ImageURL:      "https://ws-tcg.com/wordpress/wp-content/images/cardlist/d/dc_w01/dc_w01_002.png",
		Triggers:      []Trigger{TriggerSoul, TriggerGate},
		Traits:        []string{"魔法", "生徒会"},
		Text:          []string{"【自】 アンコール", "【起】 集中"},
	}
	assertCardEquals(t, card, want)
}

func TestCardFromJapaneseAPIItemMultiSideClimax(t *testing.T) {
	item := japaneseCardItem{
		CardNumber:  "Gso/WS02-124SP",
		CardName:    "巡り合う二人 キリト＆アスナ",
		CardKind:    "4",
		Color:       "[[blue.gif]]",
		CardTrigger: "[[choice.gif]]",
		Text:        "【永】 あなたのキャラすべてに、ソウルを＋2。",
		Picture:     "g/g_ws02/gso_ws02_124sp.png",
		Expansion:   10,
		Rare:        "SP",
		Side:        "-3",
	}

	card := cardFromJapaneseAPIItem(siteConfigs[Japanese], item, "電撃文庫", nil)
	if !equalSlice(card.Sides, []Side{SideWeiss, SideSchwarz}) {
		t.Fatalf("unexpected sides: %v", card.Sides)
	}
	if card.Type != CardTypeClimax {
		t.Fatalf("unexpected type: %q", card.Type)
	}
	if !equalSlice(card.Triggers, []Trigger{TriggerChoice}) {
		t.Fatalf("unexpected triggers: %v", card.Triggers)
	}
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalSlice[T comparable](sliceA []T, sliceB []T) bool {
	if len(sliceA) != len(sliceB) {
		slog.Error(fmt.Sprintf("wrong len sliceA %v, len sliceB %v", len(sliceA), len(sliceB)))
		return false
	}

	for i := range sliceA {
		if sliceA[i] != sliceB[i] {
			slog.Error(fmt.Sprintf("wrong value sliceA %v, len sliceB %v", sliceA[i], sliceB[i]))
			return false
		}
	}
	return true
}

func assertCardEquals(t *testing.T, got, want Card) {
	assertCardEqualsWithTitle(t, "", got, want)
}

func assertCardEqualsWithTitle(t *testing.T, title string, got, want Card) {
	prefix := ""
	if title != "" {
		prefix = fmt.Sprintf("[%s]: ", title)
	}
	if got.SetID != want.SetID {
		t.Errorf("%sIncorrect Set: got %q, want %q", prefix, got.SetID, want.SetID)
	}
	if got.SetName != want.SetName {
		t.Errorf("%sIncorrect SetName: got %q, want %q", prefix, got.SetName, want.SetName)
	}
	if got.ExpansionName != want.ExpansionName {
		t.Errorf("%sIncorrect ExpansionName: got %q, want %q", prefix, got.ExpansionName, want.ExpansionName)
	}
	if got.ExpansionSlug != want.ExpansionSlug {
		t.Errorf("%sIncorrect ExpansionSlug: got %q, want %q", prefix, got.ExpansionSlug, want.ExpansionSlug)
	}
	if got.ExpansionProductDisplayName != want.ExpansionProductDisplayName {
		t.Errorf("%sIncorrect ExpansionProductDisplayName: got %q, want %q", prefix, got.ExpansionProductDisplayName, want.ExpansionProductDisplayName)
	}
	if got.ExpansionProductURL != want.ExpansionProductURL {
		t.Errorf("%sIncorrect ExpansionProductURL: got %q, want %q", prefix, got.ExpansionProductURL, want.ExpansionProductURL)
	}
	if got.ExpansionSourceType != want.ExpansionSourceType {
		t.Errorf("%sIncorrect ExpansionSourceType: got %q, want %q", prefix, got.ExpansionSourceType, want.ExpansionSourceType)
	}
	if !equalSlice(got.Sides, want.Sides) {
		t.Errorf("%sIncorrect Sides: got %v, want %v", prefix, got.Sides, want.Sides)
	}
	if got.Release != want.Release {
		t.Errorf("%sIncorrect Release: got %q, want %q", prefix, got.Release, want.Release)
	}
	if got.ID != want.ID {
		t.Errorf("%sIncorrect ID: got %q, want %q", prefix, got.ID, want.ID)
	}
	if got.Name != want.Name {
		t.Errorf("%sIncorrect Name: got %q, want %q", prefix, got.Name, want.Name)
	}
	if got.Language != want.Language {
		t.Errorf("%sIncorrect Language: got %q, want %q", prefix, got.Language, want.Language)
	}
	if got.Type != want.Type {
		t.Errorf("%sIncorrect CardType: got %q, want %q", prefix, got.Type, want.Type)
	}
	if got.Color != want.Color {
		t.Errorf("%sIncorrect Colour: got %q, want %q", prefix, got.Color, want.Color)
	}
	if !equalIntPtr(got.Level, want.Level) {
		t.Errorf("%sIncorrect Level: got %v, want %v", prefix, got.Level, want.Level)
	}
	if !equalIntPtr(got.Cost, want.Cost) {
		t.Errorf("%sIncorrect Cost: got %v, want %v", prefix, got.Cost, want.Cost)
	}
	if !equalIntPtr(got.Power, want.Power) {
		t.Errorf("%sIncorrect Power: got %v, want %v", prefix, got.Power, want.Power)
	}
	if !equalIntPtr(got.Soul, want.Soul) {
		t.Errorf("%sIncorrect Soul: got %v, want %v", prefix, got.Soul, want.Soul)
	}
	if got.Rarity != want.Rarity {
		t.Errorf("%sIncorrect Rarity: got %q, want %q", prefix, got.Rarity, want.Rarity)
	}
	if got.FlavorText != want.FlavorText {
		t.Errorf("%sIncorrect FlavourText: got %q, want %q", prefix, got.FlavorText, want.FlavorText)
	}
	if !equalSlice(got.Triggers, want.Triggers) {
		t.Errorf("%sIncorrect Trigger: got %v, want %v", prefix, got.Triggers, want.Triggers)
	}
	if !equalSlice(got.ParseFailures, want.ParseFailures) {
		t.Errorf("%sIncorrect ParseFailures: got %v, want %v", prefix, got.ParseFailures, want.ParseFailures)
	}
	if !equalSlice(got.Text, want.Text) {
		t.Errorf("%sIncorrect Ability: got\n %v,\nwant\n %v", prefix, got.Text, want.Text)
	}
	if !equalSlice(got.Traits, want.Traits) {
		t.Errorf("%sIncorrect SpecialAttrib: got %v, want %v", prefix, got.Traits, want.Traits)
	}
	if got.ImageURL != want.ImageURL {
		t.Errorf("%sIncorrect ImageURL: got %q, want %q", prefix, got.ImageURL, want.ImageURL)
	}
	if got.CardNumber != want.CardNumber {
		t.Errorf("%sIncorrect Cardcode: got %q, want %q", prefix, got.CardNumber, want.CardNumber)
	}
}

func TestExtractData_jp(t *testing.T) {
	chara := `
	<th><a href="/cardlist/?cardno=BD/W63-036SPMa&amp;l"><img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/b/bd_w63/bd_w63_036spma.gif" alt="“私達、参上っ！”上原ひまり"/></a></th>
	<td>
	<h4><a href="/cardlist/?cardno=BD/W63-036SPMa&amp;l"><span>
	“私達、参上っ！”上原ひまり</span>(<span>BD/W63-036SPMa</span>)</a> -「バンドリ！ ガールズバンドパーティ！」Vol.2<br/></h4>
	<span class="unit">
	サイド：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/w.gif"/></span>
	<span class="unit">種類：キャラ</span>
	<span class="unit">レベル：2</span><br/>
	<span class="unit">色：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/green.gif"/></span>
	<span class="unit">パワー：6000</span>
	<span class="unit">ソウル：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/soul.gif"/><img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/soul.gif"/></span>
	<span class="unit">コスト：1</span><br/>
	<span class="unit">レアリティ：SPMa</span>
	<span class="unit">トリガー：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/soul.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/bounce.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/shot.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/treasure.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/standby.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/salvage.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/gate.gif"/>
	<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/draw.gif"/>
	</span>
	<span class="unit">特徴：<span>音楽・Afterglow</span></span><br/>
	<span class="unit">フレーバー：-</span><br/>
	<br/>
	<span>【永】 あなたのターン中、他のあなたの「“止まらずに、前へ”美竹蘭」がいるなら、このカードのパワーを＋6000。<br/>【自】［(1)］ このカードがアタックした時 、あなたはコストを払ってよい。そうしたら、そのアタック中、あなたはトリガーステップにトリガーチェックを2回行う。</span>
	</td>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chara))
	expectedTrigger := []Trigger{TriggerSoul, TriggerReturn, TriggerShot, TriggerTreasure, TriggerStandby, TriggerComeback, TriggerGate, TriggerDraw}
	expectedTrait := []string{"音楽", "Afterglow"}
	expectedAbility := []string{
		"【永】 あなたのターン中、他のあなたの「“止まらずに、前へ”美竹蘭」がいるなら、このカードのパワーを＋6000。",
		"【自】［(1)］ このカードがアタックした時 、あなたはコストを払ってよい。そうしたら、そのアタック中、あなたはトリガーステップにトリガーチェックを2回行う。",
	}

	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	if card.Name != "“私達、参上っ！”上原ひまり" {
		t.Errorf("got %v: expected “私達、参上っ！”上原ひまり", card.Name)
	}
	if card.SetID != "BD" {
		t.Errorf("got %v: expected BD", card.SetID)
	}
	if !equalSlice(card.Sides, []Side{SideWeiss}) {
		t.Errorf("got %v: expected [W]", card.Sides)
	}
	if card.Release != "W63" {
		t.Errorf("got %v: expected W63", card.Release)
	}
	if card.ID != "036SPMa" {
		t.Errorf("got %v: expected 036SPMa", card.ID)
	}
	if !equalIntPtr(card.Level, intPtr(2)) {
		t.Errorf("got %v: expected 2", card.Level)
	}
	if card.Color != "GREEN" {
		t.Errorf("got %v: expected GREEN", card.Color)
	}
	if !equalIntPtr(card.Power, intPtr(6000)) {
		t.Errorf("got %v: expected 6000", card.Power)
	}
	if !equalIntPtr(card.Soul, intPtr(2)) {
		t.Errorf("got %v: expected 2", card.Soul)
	}
	if !equalIntPtr(card.Cost, intPtr(1)) {
		t.Errorf("got %v: expected 1", card.Cost)
	}
	if card.Type != "CH" {
		t.Errorf("got %v: expected CH", card.Type)
	}
	if card.Rarity != "SPMa" {
		t.Errorf("got %v: expected SPMa", card.Rarity)
	}
	if !equalSlice(card.Triggers, expectedTrigger) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTrigger)
	}
	if !equalSlice(card.Traits, expectedTrait) {
		t.Errorf("got %v: expected %v", card.Traits, expectedTrait)
	}
	if !equalSlice(card.Text, expectedAbility) {
		t.Errorf("got \n %v: expected \n %v", card.Text, expectedAbility)
	}
}

func TestExtractData_jp_purple(t *testing.T) {
	html := `
	<tr>
<th><a href="/cardlist/?cardno=PY/S38-125&amp;l"><img src="/wordpress/wp-content/images/cardlist/p/py_s38/py_s38_125.png" alt="むらさきパプリス"></a></th>
<td>
<h4><a href="/cardlist/?cardno=PY/S38-125&amp;l" class=""><span class="highlight_target">
むらさきパプリス</span>(<span class="highlight_target"><span class="highlight">PY/S38-125</span></span>)</a> -PRカード【Sサイド】<br></h4>
<span class="unit">
サイド：<img src="/wordpress/wp-content/images/cardlist/_partimages/s.gif"></span>
<span class="unit">種類：キャラ</span>
<span class="unit">レベル：0</span><br>
<span class="unit">色：紫</span>
<span class="unit">パワー：1000</span>
<span class="unit">ソウル：<img src="/wordpress/wp-content/images/cardlist/_partimages/soul.gif"></span>
<span class="unit">コスト：0</span><br>
<span class="unit">レアリティ：PR</span>
<span class="unit">トリガー：-</span>
<span class="unit">特徴：<span class="highlight_target">ぷよ・動物</span></span><br>
<span class="unit">フレーバー：むらさきパプリスが、<br>いっちばんかわいいでしょー！</span><br>
<br>
<span class="highlight_target">【永】 応援 このカードの前のあなたのキャラすべてに、パワーを＋500。</span>
</td>
</tr>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	expectedCard := Card{
		Name:          "むらさきパプリス",
		SetID:         "PY",
		ExpansionName: "PRカード【Sサイド】",
		Sides:         []Side{SideSchwarz},
		CardNumber:    "PY/S38-125",
		Release:       "S38",
		ReleasePackID: "38",
		ID:            "125",
		Color:         "PURPLE",
		Language:      "ja",
		Type:          "CH",
		Soul:          intPtr(1),
		Level:         intPtr(0),
		Cost:          intPtr(0),
		FlavorText:    "むらさきパプリスが、いっちばんかわいいでしょー！",
		Power:         intPtr(1000),
		Rarity:        "PR",
		ImageURL:      "https://ws-tcg.com/wordpress/wp-content/images/cardlist/p/py_s38/py_s38_125.png",
		Traits:        []string{"ぷよ", "動物"},
		Text:          []string{"【永】 応援 このカードの前のあなたのキャラすべてに、パワーを＋500。"},
	}
	assertCardEquals(t, card, expectedCard)
}

func TestExtractData_jp_multiSideCard(t *testing.T) {
	html := `
	<tr>
<th><a href="/cardlist/?cardno=Gso/WS02-124SP&amp;l"><img src="/wordpress/wp-content/images/cardlist/g/g_ws02/gso_ws02_124sp.png" alt="巡り合う二人 キリト＆アスナ"></a></th>
<td>
<h4><a href="/cardlist/?cardno=Gso/WS02-124SP&amp;l"><span class="highlight_target">
巡り合う二人 キリト＆アスナ</span>(<span class="highlight_target"><span class="highlight">Gso/WS02-124SP</span></span>)</a> -電撃文庫<br></h4>
<span class="unit">
サイド：<img src="/wordpress/wp-content/images/cardlist/_partimages/w.gif"> <img src="/wordpress/wp-content/images/cardlist/_partimages/s.gif"></span>
<span class="unit">種類：キャラ</span>
<span class="unit">レベル：1</span><br>
<span class="unit">色：<img src="/wordpress/wp-content/images/cardlist/_partimages/blue.gif"></span>
<span class="unit">パワー：4000</span>
<span class="unit">ソウル：<img src="/wordpress/wp-content/images/cardlist/_partimages/soul.gif"></span>
<span class="unit">コスト：0</span><br>
<span class="unit">レアリティ：SP</span>
<span class="unit">トリガー：-</span>
<span class="unit">特徴：<span class="highlight_target">電撃文庫・アバター・武器</span></span><br>
<span class="unit">フレーバー：-</span><br>
<br>
<span class="highlight_target">【自】 このカードが手札から舞台に置かれた時、他のあなたの、《電撃文庫》か《アバター》か《ネット》のキャラがいるなら、そのターン中、このカードのパワーを＋2000。<br>【自】 加速 ［(1) あなたの山札の上から1枚をクロック置場に置き、手札を1枚控え室に置く］ このカードがアタックした時、あなたはコストを払ってよい。そうしたら、あなたは自分の山札を見て《電撃文庫》か《アバター》か《ネット》のキャラを2枚まで選んで相手に見せ、手札に加え、その山札をシャッフルする。</span>
</td>
</tr>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	expectedCard := Card{
		Name:          "巡り合う二人 キリト＆アスナ",
		SetID:         "Gso",
		ExpansionName: "電撃文庫",
		Sides:         []Side{SideWeiss, SideSchwarz},
		CardNumber:    "Gso/WS02-124SP",
		Release:       "WS02",
		ReleasePackID: "02",
		ID:            "124SP",
		Color:         "BLUE",
		Language:      "ja",
		Type:          "CH",
		Soul:          intPtr(1),
		Level:         intPtr(1),
		Cost:          intPtr(0),
		FlavorText:    "-",
		Power:         intPtr(4000),
		Rarity:        "SP",
		ImageURL:      "https://ws-tcg.com/wordpress/wp-content/images/cardlist/g/g_ws02/gso_ws02_124sp.png",
		Traits:        []string{"電撃文庫", "アバター", "武器"},
		Triggers:      []Trigger{},
		Text: []string{
			"【自】 このカードが手札から舞台に置かれた時、他のあなたの、《電撃文庫》か《アバター》か《ネット》のキャラがいるなら、そのターン中、このカードのパワーを＋2000。",
			"【自】 加速 ［(1) あなたの山札の上から1枚をクロック置場に置き、手札を1枚控え室に置く］ このカードがアタックした時、あなたはコストを払ってよい。そうしたら、あなたは自分の山札を見て《電撃文庫》か《アバター》か《ネット》のキャラを2枚まで選んで相手に見せ、手札に加え、その山札をシャッフルする。",
		},
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	assertCardEquals(t, card, expectedCard)
}

func TestExtractDataEvent_jp(t *testing.T) {
	chara := `
	<th><a href="/cardlist/?cardno=BD/W63-022&amp;l"><img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/b/bd_w63/bd_w63_022.gif" alt="ミッシェルからの伝言"></a></th>
	<td>
	<h4><a href="/cardlist/?cardno=BD/W63-022&amp;l"><span class="highlight_target">
	ミッシェルからの伝言</span>(<span class="highlight_target">BD/W63-022</span>)</a> -「バンドリ！ ガールズバンドパーティ！」Vol.2<br></h4>
	<span class="unit">
	サイド：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/w.gif"></span>
	<span class="unit">種類：イベント</span>
	<span class="unit">レベル：1</span><br>
	<span class="unit">色：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/yellow.gif"></span>
	<span class="unit">パワー：-</span>
	<span class="unit">ソウル：-</span>
	<span class="unit">コスト：0</span><br>
	<span class="unit">レアリティ：U</span>
	<span class="unit">トリガー：－</span>
	<span class="unit">特徴：<span class="highlight_target">-・-</span></span><br>
	<span class="unit">フレーバー：美咲「あはは……ありがとう、はぐみ」</span><br>
	<br>
	<span class="highlight_target">このカードは、あなたの《ハロー、ハッピーワールド！》のキャラが2枚以下なら、手札からプレイできない。<br>あなたは自分の山札の上から2枚を、控え室に置き、自分の控え室のレベルＸ以下のキャラを1枚選び、手札に戻す。Ｘはそれらのカードのレベルの合計に等しい。（クライマックスのレベルは0として扱う）</span>
	</td>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chara))
	var expectedTrigger []Trigger

	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	if card.Name != "ミッシェルからの伝言" {
		t.Errorf("got %v: expected ミッシェルからの伝言", card.Name)
	}

	if !equalSlice(card.Triggers, expectedTrigger) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTrigger)
	}

	if card.Type != "EV" {
		t.Errorf("got %v: expected EV", card.Type)
	}

	if !equalSlice(card.Traits, []string{}) {
		t.Errorf("got %v: expected empty", card.Traits)
	}

	if card.Soul != nil {
		t.Errorf("got %v: expected nil", card.Soul)
	}

	if card.Power != nil {
		t.Errorf("got %v: expected nil", card.Power)
	}
}

func TestExtractDataCX_jp(t *testing.T) {
	chara := `
<tr>
	<th><a href="/cardlist/?cardno=BD/W63-025&amp;l"><img src="/wordpress/wp-content/images/cardlist/b/bd_w63/bd_w63_025.png" alt="キラキラのお日様"></a></th>
	<td>
	<h4><a href="/cardlist/?cardno=BD/W63-025&amp;l"><span class="highlight_target">
	キラキラのお日様</span>(<span class="highlight_target">BD/W63-025</span>)</a> -「バンドリ！ ガールズバンドパーティ！」Vol.2<br></h4>
	<span class="unit">
	サイド：<img src="/wordpress/wp-content/images/cardlist/_partimages/w.gif"></span>
	<span class="unit">種類：クライマックス</span>
	<span class="unit">レベル：-</span><br>
	<span class="unit">色：<img src="/wordpress/wp-content/images/cardlist/_partimages/yellow.gif"></span>
	<span class="unit">パワー：-</span>
	<span class="unit">ソウル：-</span>
	<span class="unit">コスト：-</span><br>
	<span class="unit">レアリティ：CR</span>
	<span class="unit">トリガー：<img src="/wordpress/wp-content/images/cardlist/_partimages/soul.gif"><img src="/wordpress/wp-content/images/cardlist/_partimages/bounce.gif"></span>
	<span class="unit">特徴：<span class="highlight_target">-</span></span><br>
	<span class="unit">フレーバー：楽しい気持ちは誰かといると生まれるものってこと！</span><br>
	<br>
	<span class="highlight_target">【永】 あなたのキャラすべてに、パワーを＋1000し、ソウルを＋1。<br>（<img src="/wordpress/wp-content/images/cardlist/_partimages/bounce.gif">：このカードがトリガーした時、あなたは相手のキャラを1枚選び、手札に戻してよい）</span>
	</td>
</tr>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chara))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)

	expectedCard := Card{
		Name:          "キラキラのお日様",
		SetID:         "BD",
		ExpansionName: "「バンドリ！ ガールズバンドパーティ！」Vol.2",
		Sides:         []Side{SideWeiss},
		CardNumber:    "BD/W63-025",
		Release:       "W63",
		ReleasePackID: "63",
		ID:            "025",
		Color:         "YELLOW",
		Language:      "ja",
		Type:          "CX",
		Soul:          nil,
		Level:         nil,
		Cost:          nil,
		FlavorText:    "楽しい気持ちは誰かといると生まれるものってこと！",
		Power:         nil,
		Rarity:        "CR",
		ImageURL:      "https://ws-tcg.com/wordpress/wp-content/images/cardlist/b/bd_w63/bd_w63_025.png",
		Triggers:      []Trigger{TriggerSoul, TriggerReturn},
		Text: []string{
			"【永】 あなたのキャラすべてに、パワーを＋1000し、ソウルを＋1。",
			"（[RETURN]：このカードがトリガーした時、あなたは相手のキャラを1枚選び、手札に戻してよい）",
		},
	}
	assertCardEquals(t, card, expectedCard)
}

func TestExtractData_en(t *testing.T) {
	chara := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/f/fs_s64/FS_BCS_2019_03.png" alt="EGOISTIC, Sakura" decoding="async">
		</div>
		<div class="p-cards__detail-textarea">
		<p class="number">FS/BCS2019-03</p>
		<p class="ttl u-mt-14 u-mt-16-sp">EGOISTIC, Sakura</p>
		<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Expansion</dt>
			<dd>PR Card 【Schwarz Side】</dd>
			</dl>
			<dl>
			<dt>Traits</dt>
			<dd>Master・Love</dd>
			</dl>
			<dl>
			<dt>Card Type</dt>
			<dd>Character</dd>
			</dl>
			<dl>
			<dt>Rarity</dt>
			<dd>PR</dd>
			</dl>
			<dl>
			<dt>Side</dt>
			<dd>
								<img src="/cardlist/partimages/s.gif" alt="" decoding="async">
								</dd>
			</dl>
			<dl>
			<dt>Color</dt>
			<dd><img src="/wp/wp-content/images/partimages/green.gif"></dd>
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
			<dd>2000</dd>
			</dl>
			<dl>
			<dt>Trigger</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Soul</dt>
			<dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail u-mt-22 u-mt-40-sp">
			<p>【AUTO】 When this card is placed on the stage from your hand, choose 1 of your 《Master》 or 《Servant》 characters, and that character gets +1500 power until end of turn.</p>
		</div>
		<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
			<p>I wish someone like this didn't exist.</p>
		</div>
		<p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©TYPE-MOON, ufotable, FSNPC</p>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chara))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	expectedCard := Card{
		Name:          "EGOISTIC, Sakura",
		ExpansionName: "PR Card 【Schwarz Side】",
		CardNumber:    "FS/BCS2019-03",
		SetID:         "FS",
		Sides:         []Side{SideSchwarz},
		Release:       "BCS2019",
		ReleasePackID: "2019",
		ID:            "03",
		Level:         intPtr(0),
		Color:         "GREEN",
		Power:         intPtr(2000),
		Soul:          intPtr(1),
		Cost:          intPtr(0),
		Language:      "en",
		Type:          "CH",
		Rarity:        "PR",
		FlavorText:    "I wish someone like this didn't exist.",
		Traits:        []string{"Master", "Love"},
		Text:          []string{"【AUTO】 When this card is placed on the stage from your hand, choose 1 of your 《Master》 or 《Servant》 characters, and that character gets +1500 power until end of turn."},
		ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/f/fs_s64/FS_BCS_2019_03.png",
	}
	assertCardEquals(t, card, expectedCard)
}

func TestExtractData_en_multiIconAbility(t *testing.T) {
	character := `
<div class="c-header">
	<nav><a href="https://en.ws-tcg.com/products/">Products</a></nav>
</div>
<div class="l-subpage__contents-max u-mt-80 u-mt-60-sp">
	<div class="p-cards__detail-wrapper">
		<div class="p-cards__detail-wrapper-inner">
			<div class="image"><img src="/wp/wp-content/images/cardimages/ATLA/BP/ATLA_WX04_007S.png" alt="Aang: Learning Avatar State" decoding="async">
			</div>
			<div class="p-cards__detail-textarea">
			<p class="number">ATLA/WX04-007S</p>
			<p class="ttl u-mt-14 u-mt-16-sp">Aang: Learning Avatar State</p>
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
				<dd>SR</dd>
				</dl>
				<dl>
				<dt>Side</dt>
				<dd>
									<img src="/cardlist/partimages/w.gif" alt="" decoding="async">
									</dd>
				</dl>
				<dl>
				<dt>Color</dt>
				<dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd>
				</dl>
			</div>
			<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
				<dl>
				<dt>Level</dt>
				<dd>2</dd>
				</dl>
				<dl>
				<dt>Cost</dt>
				<dd>1</dd>
				</dl>
				<dl>
				<dt>Power</dt>
				<dd>1000</dd>
				</dl>
				<dl>
				<dt>Trigger</dt>
				<dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
				</dl>
				<dl>
				<dt>Soul</dt>
				<dd>-</dd>
				</dl>
			</div>
			<div class="p-cards__detail u-mt-22 u-mt-40-sp">
				<p>【CONT】 If your climax area has a climax with <img src="/wp/wp-content/images/partimages/choice.gif"> in its trigger icon, this card in all of your zones get <img src="/wp/wp-content/images/partimages/choice.gif"> in the trigger icon. If there is a climax with <img src="/wp/wp-content/images/partimages/treasure.gif"> in its trigger icon, this card in all of your zones get <img src="/wp/wp-content/images/partimages/treasure.gif"> in the trigger icon. If there is a climax with <img src="/wp/wp-content/images/partimages/standby.gif"> in its trigger icon, this card in all of your zones get <img src="/wp/wp-content/images/partimages/standby.gif"> in the trigger icon. If there is a climax with <img src="/wp/wp-content/images/partimages/gate.gif"> in its trigger icon, this card in all of your zones get <img src="/wp/wp-content/images/partimages/gate.gif"> in the trigger icon.<br>【AUTO】 【CLOCK】 Alarm If this card is the top card of your clock, and you have 4 or more 《World of Avatar》 characters, at the beginning of your climax phase, you may put the top card of your deck into your stock.</p>
			</div>
			<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
				<p>-</p>
			</div>
			<p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©2023 Viacom International Inc. All Rights Reserved.</p>
			</div>
		</div>
	</div>
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
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(character))
	if err != nil {
		t.Fatal(err)
	}

	expectedCard := Card{
		CardNumber:          "ATLA/WX04-007S",
		SetID:               "ATLA",
		ExpansionName:       "Avatar: The Last Airbender",
		Sides:               []Side{SideWeiss},
		Release:             "WX04",
		ReleasePackID:       "WX",
		ID:                  "007S",
		Language:            "en",
		Type:                "CH",
		Name:                "Aang: Learning Avatar State",
		Color:               "YELLOW",
		Soul:                intPtr(0),
		Level:               intPtr(2),
		Cost:                intPtr(1),
		FlavorText:          "",
		Power:               intPtr(1000),
		Rarity:              "SR",
		ImageURL:            "https://en.ws-tcg.com/wp/wp-content/images/cardimages/ATLA/BP/ATLA_WX04_007S.png",
		ExpansionSlug:       "bp-atla",
		ExpansionProductURL: "https://en.ws-tcg.com/products/bp-atla/",
		ExpansionSourceType: ExpansionSourceTypeProductPage,
		Triggers:            []Trigger{TriggerSoul},
		Traits:              []string{"World of Avatar", "Air Nomads"},
		Text: []string{
			"【CONT】 If your climax area has a climax with [CHOICE] in its trigger icon, this card in all of your zones get [CHOICE] in the trigger icon. If there is a climax with [TREASURE] in its trigger icon, this card in all of your zones get [TREASURE] in the trigger icon. If there is a climax with [STANDBY] in its trigger icon, this card in all of your zones get [STANDBY] in the trigger icon. If there is a climax with [GATE] in its trigger icon, this card in all of your zones get [GATE] in the trigger icon.",
			"【AUTO】 【CLOCK】 Alarm If this card is the top card of your clock, and you have 4 or more 《World of Avatar》 characters, at the beginning of your climax phase, you may put the top card of your deck into your stock.",
		},
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	assertCardEquals(t, card, expectedCard)
}

func TestExtractData_en_newTriggers(t *testing.T) {
	html := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/TST/TST_W01_001.png" alt="New Trigger Test" decoding="async"></div>
		<div class="p-cards__detail-textarea">
			<p class="number">TST/W01-001</p>
			<p class="ttl u-mt-14 u-mt-16-sp">New Trigger Test</p>
			<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
				<dl><dt>Expansion</dt><dd>Trigger Test</dd></dl>
				<dl><dt>Traits</dt><dd>Test</dd></dl>
				<dl><dt>Card Type</dt><dd>Climax</dd></dl>
				<dl><dt>Rarity</dt><dd>CX</dd></dl>
				<dl><dt>Side</dt><dd><img src="/cardlist/partimages/w.gif" alt="" decoding="async"></dd></dl>
				<dl><dt>Color</dt><dd><img src="/wp/wp-content/images/partimages/blue.gif"></dd></dl>
			</div>
			<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
				<dl><dt>Level</dt><dd>-</dd></dl>
				<dl><dt>Cost</dt><dd>-</dd></dl>
				<dl><dt>Power</dt><dd>-</dd></dl>
				<dl><dt>Trigger</dt><dd><img src="/wp/wp-content/images/partimages/discovery.gif"><img src="/wp/wp-content/images/partimages/chance.gif"></dd></dl>
				<dl><dt>Soul</dt><dd>-</dd></dl>
			</div>
			<div class="p-cards__detail u-mt-22 u-mt-40-sp">
				<p>【CONT】 All of your characters get +1000 power.<br>(<img src="/wp/wp-content/images/partimages/chance.gif">: Trigger test text)</p>
			</div>
			<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
				<p>-</p>
			</div>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	expectedTriggers := []Trigger{TriggerDiscovery, TriggerChance}
	if !equalSlice(card.Triggers, expectedTriggers) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTriggers)
	}

	expectedAbility := []string{
		"【CONT】 All of your characters get +1000 power.",
		"([CHANCE]: Trigger test text)",
	}
	if !equalSlice(card.Text, expectedAbility) {
		t.Errorf("got %v: expected %v", card.Text, expectedAbility)
	}
}

func TestExtractData_en_comebackTrigger(t *testing.T) {
	html := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/TST/TST_W01_002.png" alt="Comeback Trigger Test" decoding="async"></div>
		<div class="p-cards__detail-textarea">
			<p class="number">TST/W01-002</p>
			<p class="ttl u-mt-14 u-mt-16-sp">Comeback Trigger Test</p>
			<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
				<dl><dt>Expansion</dt><dd>Trigger Test</dd></dl>
				<dl><dt>Traits</dt><dd>Test</dd></dl>
				<dl><dt>Card Type</dt><dd>Climax</dd></dl>
				<dl><dt>Rarity</dt><dd>CX</dd></dl>
				<dl><dt>Side</dt><dd><img src="/cardlist/partimages/w.gif" alt="" decoding="async"></dd></dl>
				<dl><dt>Color</dt><dd><img src="/wp/wp-content/images/partimages/red.gif"></dd></dl>
			</div>
			<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
				<dl><dt>Level</dt><dd>-</dd></dl>
				<dl><dt>Cost</dt><dd>-</dd></dl>
				<dl><dt>Power</dt><dd>-</dd></dl>
				<dl><dt>Trigger</dt><dd><img src="/wp/wp-content/images/partimages/comeback.gif"></dd></dl>
				<dl><dt>Soul</dt><dd>-</dd></dl>
			</div>
			<div class="p-cards__detail u-mt-22 u-mt-40-sp">
				<p>【CONT】 All of your characters get +1000 power.<br>(<img src="/wp/wp-content/images/partimages/comeback.gif">: Return 1 character from your waiting room to your hand)</p>
			</div>
			<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
				<p>-</p>
			</div>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	expectedTriggers := []Trigger{TriggerComeback}
	if !equalSlice(card.Triggers, expectedTriggers) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTriggers)
	}

	expectedAbility := []string{
		"【CONT】 All of your characters get +1000 power.",
		"([COMEBACK]: Return 1 character from your waiting room to your hand)",
	}
	if !equalSlice(card.Text, expectedAbility) {
		t.Errorf("got %v: expected %v", card.Text, expectedAbility)
	}
}

func TestExtractData_jp_newTriggers(t *testing.T) {
	html := `
<tr>
<th><a href="/cardlist/?cardno=TST/W01-001&amp;l"><img src="/wordpress/wp-content/images/cardlist/t/tst_w01/tst_w01_001.png" alt="新トリガーテスト"></a></th>
<td>
<h4><a href="/cardlist/?cardno=TST/W01-001&amp;l"><span>新トリガーテスト</span>(<span>TST/W01-001</span>)</a> -トリガーテスト<br></h4>
<span class="unit">サイド：<img src="/wordpress/wp-content/images/cardlist/_partimages/w.gif"></span>
<span class="unit">種類：クライマックス</span>
<span class="unit">レベル：-</span><br>
<span class="unit">色：<img src="/wordpress/wp-content/images/cardlist/_partimages/blue.gif"></span>
<span class="unit">パワー：-</span>
<span class="unit">ソウル：-</span>
<span class="unit">コスト：-</span><br>
<span class="unit">レアリティ：CX</span>
<span class="unit">トリガー：<img src="/wordpress/wp-content/images/cardlist/_partimages/discovery.gif"><img src="/wordpress/wp-content/images/cardlist/_partimages/chance.gif"></span>
<span class="unit">特徴：<span>-</span></span><br>
<span class="unit">フレーバー：-</span><br>
<br>
<span>【永】 あなたのキャラすべてに、パワーを＋1000。<br>（<img src="/wordpress/wp-content/images/cardlist/_partimages/discovery.gif">：トリガーテスト）</span>
</td>
</tr>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	expectedTriggers := []Trigger{TriggerDiscovery, TriggerChance}
	if !equalSlice(card.Triggers, expectedTriggers) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTriggers)
	}

	expectedAbility := []string{
		"【永】 あなたのキャラすべてに、パワーを＋1000。",
		"（[DISCOVERY]：トリガーテスト）",
	}
	if !equalSlice(card.Text, expectedAbility) {
		t.Errorf("got %v: expected %v", card.Text, expectedAbility)
	}
}

func TestExtractData_en_multiSideCard(t *testing.T) {
	cardHTML := `
<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wordpress/wp-content/images/cardimages/Gxx/WS02_E124SP.png" alt="The Two Who Meet by Chance, Kirito &amp; Asuna" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">Gso/WS02-E124SP</p>
            <p class="ttl u-mt-14 u-mt-16-sp">The Two Who Meet by Chance, Kirito &amp; Asuna</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>Dengeki Bunko</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Dengeki Bunko・Avatar・Weapon</dd>
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
                  <img src="/cardlist/partimages/s.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wordpress/wp-content/images/partimages/blue.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>1</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>0</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>4000</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>－</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wordpress/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 When this card is placed on the stage from your hand, if you have another 《Dengeki Bunko》 or 《Avatar》 or 《Net》 character, this card gets +2000 power until end of turn.<br>【AUTO】 Accelerate [(1) Put the top card of your deck into your clock &amp; Put 1 card from your hand into your waiting room] When this card attacks, you may pay the cost. If you do, search your deck for up to 2 《Dengeki Bunko》 or 《Avatar》 or 《Net》 characters, reveal them to your opponent, put them into your hand, and shuffle your deck.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>-</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©KADOKAWA CORPORATION 2024　©Reki Kawahara 2024　illustration/abec</p>
          </div>
        </div>
      </div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(cardHTML))
	if err != nil {
		t.Fatal(err)
	}

	expectedCard := Card{
		CardNumber:    "Gso/WS02-E124SP",
		SetID:         "Gso",
		ExpansionName: "Dengeki Bunko",
		Sides:         []Side{SideWeiss, SideSchwarz},
		Release:       "WS02",
		ReleasePackID: "02",
		ID:            "E124SP",
		Language:      "en",
		Type:          "CH",
		Name:          "The Two Who Meet by Chance, Kirito & Asuna",
		Color:         "BLUE",
		Soul:          intPtr(1),
		Level:         intPtr(1),
		Cost:          intPtr(0),
		FlavorText:    "",
		Power:         intPtr(4000),
		Rarity:        "SP",
		ImageURL:      "https://en.ws-tcg.com/wordpress/wp-content/images/cardimages/Gxx/WS02_E124SP.png",
		Triggers:      []Trigger{},
		Traits:        []string{"Dengeki Bunko", "Avatar", "Weapon"},
		Text: []string{
			"【AUTO】 When this card is placed on the stage from your hand, if you have another 《Dengeki Bunko》 or 《Avatar》 or 《Net》 character, this card gets +2000 power until end of turn.",
			"【AUTO】 Accelerate [(1) Put the top card of your deck into your clock & Put 1 card from your hand into your waiting room] When this card attacks, you may pay the cost. If you do, search your deck for up to 2 《Dengeki Bunko》 or 《Avatar》 or 《Net》 characters, reveal them to your opponent, put them into your hand, and shuffle your deck.",
		},
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	assertCardEquals(t, card, expectedCard)
}

func TestExtractData_en_triggerParseFailureIsNonFatal(t *testing.T) {
	html := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/TST/TST_W01_003.png" alt="Trigger Failure Test" decoding="async"></div>
		<div class="p-cards__detail-textarea">
			<p class="number">TST/W01-003</p>
			<p class="ttl u-mt-14 u-mt-16-sp">Trigger Failure Test</p>
			<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
				<dl><dt>Expansion</dt><dd>Trigger Test</dd></dl>
				<dl><dt>Traits</dt><dd>Test・Benign</dd></dl>
				<dl><dt>Card Type</dt><dd>Character</dd></dl>
				<dl><dt>Rarity</dt><dd>R</dd></dl>
				<dl><dt>Side</dt><dd><img src="/cardlist/partimages/w.gif" alt="" decoding="async"></dd></dl>
				<dl><dt>Color</dt><dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd></dl>
			</div>
			<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
				<dl><dt>Level</dt><dd>1</dd></dl>
				<dl><dt>Cost</dt><dd>0</dd></dl>
				<dl><dt>Power</dt><dd>5000</dd></dl>
				<dl><dt>Trigger</dt><dd><img src="/wp/wp-content/images/partimages/yellow.gif"><img src="/wp/wp-content/images/partimages/soul.gif"></dd></dl>
				<dl><dt>Soul</dt><dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd></dl>
			</div>
			<div class="p-cards__detail u-mt-22 u-mt-40-sp">
				<p>【AUTO】 When this card attacks, this card gets +1000 power until end of turn.</p>
			</div>
			<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
				<p>-</p>
			</div>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)
	expectedCard := Card{
		CardNumber:    "TST/W01-003",
		SetID:         "TST",
		ExpansionName: "Trigger Test",
		Sides:         []Side{SideWeiss},
		Release:       "W01",
		ReleasePackID: "01",
		ID:            "003",
		Language:      "en",
		Type:          "CH",
		Name:          "Trigger Failure Test",
		Color:         "YELLOW",
		Soul:          intPtr(1),
		Level:         intPtr(1),
		Cost:          intPtr(0),
		Power:         intPtr(5000),
		Rarity:        "R",
		ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/TST/TST_W01_003.png",
		Triggers:      []Trigger{TriggerSoul},
		ParseFailures: []string{"unknown trigger icon: yellow"},
		Traits:        []string{"Test", "Benign"},
		Text: []string{
			"【AUTO】 When this card attacks, this card gets +1000 power until end of turn.",
		},
	}

	assertCardEquals(t, card, expectedCard)
}

func TestExtractDataEvent_en(t *testing.T) {
	event := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/SS/WE41_E17.png" alt="The Day Yuji Disappeared" decoding="async">
		</div>
		<div class="p-cards__detail-textarea">
		<p class="number">SS/WE41-E17</p>
		<p class="ttl u-mt-14 u-mt-16-sp">The Day Yuji Disappeared</p>
		<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Expansion</dt>
			<dd>[EX] Shakugan no Shana</dd>
			</dl>
			<dl>
			<dt>Traits</dt>
			<dd></dd>
			</dl>
			<dl>
			<dt>Card Type</dt>
			<dd>Event</dd>
			</dl>
			<dl>
			<dt>Rarity</dt>
			<dd>N</dd>
			</dl>
			<dl>
			<dt>Side</dt>
			<dd>
								<img src="/cardlist/partimages/w.gif" alt="" decoding="async">
								</dd>
			</dl>
			<dl>
			<dt>Color</dt>
			<dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Level</dt>
			<dd>2</dd>
			</dl>
			<dl>
			<dt>Cost</dt>
			<dd>1</dd>
			</dl>
			<dl>
			<dt>Power</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Trigger</dt>
			<dd>－</dd>
			</dl>
			<dl>
			<dt>Soul</dt>
			<dd>-</dd>
			</dl>
		</div>
		<div class="p-cards__detail u-mt-22 u-mt-40-sp">
			<p>Search your deck for up to 2 《Flame》 characters, reveal them to your opponent, put them into your hand, choose 1 card in your hand, put it into your waiting room, and shuffle your deck.<br>Put this card into your memory.<br></p>
		</div>
		<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
			<p>Yuji...</p>
		</div>
		<p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">© YASHICHIRO TAKAHASHI/NOIZI ITO/ASCII MEDIA WORKS/「Shakugan no Shana F」committee</p>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(event))

	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)

	if card.Type != "EV" {
		t.Errorf("got %v: expected EV", card.Type)
	}

	if card.Name != "The Day Yuji Disappeared" {
		t.Errorf("got %v: expected The Day Yuji Disappeared", card.Name)
	}

	var expectedTrigger []Trigger
	if !equalSlice(card.Triggers, expectedTrigger) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTrigger)
	}

	if !equalSlice(card.Traits, []string{}) {
		t.Errorf("got %v: expected empty", card.Traits)
	}

	if !equalIntPtr(card.Level, intPtr(2)) {
		t.Errorf("got %v: expected 2", card.Level)
	}

	if card.Color != "YELLOW" {
		t.Errorf("got %v: expected YELLOW", card.Color)
	}

	if card.Soul != nil {
		t.Errorf("got %v: expected nil", card.Soul)
	}

	if card.Power != nil {
		t.Errorf("got %v: expected nil", card.Power)
	}
}

// The English site sometimes labels a card as Event or Climax while still listing soul icons.
// Soul should reflect the markup, not the stated card type.
func TestExtractData_soulFromMarkupWhenCardTypeNotCharacter_en(t *testing.T) {
	t.Run("event_with_soul_icons", func(t *testing.T) {
		html := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/X/EVTEST.png" alt="Mislabeled Event" decoding="async">
		</div>
		<div class="p-cards__detail-textarea">
		<p class="number">XX/W99-E01</p>
		<p class="ttl u-mt-14 u-mt-16-sp">Mislabeled Event</p>
		<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Expansion</dt>
			<dd>[TEST] Expansion</dd>
			</dl>
			<dl>
			<dt>Traits</dt>
			<dd></dd>
			</dl>
			<dl>
			<dt>Card Type</dt>
			<dd>Event</dd>
			</dl>
			<dl>
			<dt>Rarity</dt>
			<dd>C</dd>
			</dl>
			<dl>
			<dt>Side</dt>
			<dd>
								<img src="/cardlist/partimages/w.gif" alt="" decoding="async">
								</dd>
			</dl>
			<dl>
			<dt>Color</dt>
			<dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd>
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
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Trigger</dt>
			<dd>－</dd>
			</dl>
			<dl>
			<dt>Soul</dt>
			<dd><img src="/wp/wp-content/images/partimages/soul.gif"><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail u-mt-22 u-mt-40-sp">
			<p>Effect text.</p>
		</div>
		</div>
	</div>
</div>
`
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatal(err)
		}
		card := extractData(siteConfigs[English], doc.Clone(), nil)
		if card.Type != CardTypeEvent {
			t.Fatalf("Type: got %q want EV", card.Type)
		}
		if !equalIntPtr(card.Soul, intPtr(2)) {
			t.Fatalf("Soul: got %v want 2", card.Soul)
		}
	})
	t.Run("climax_with_soul_icons", func(t *testing.T) {
		html := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/X/CXTEST.png" alt="Mislabeled CX" decoding="async">
		</div>
		<div class="p-cards__detail-textarea">
		<p class="number">XX/W99-099</p>
		<p class="ttl u-mt-14 u-mt-16-sp">Mislabeled CX</p>
		<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Expansion</dt>
			<dd>[TEST] Expansion</dd>
			</dl>
			<dl>
			<dt>Traits</dt>
			<dd></dd>
			</dl>
			<dl>
			<dt>Card Type</dt>
			<dd>Climax</dd>
			</dl>
			<dl>
			<dt>Rarity</dt>
			<dd>CC</dd>
			</dl>
			<dl>
			<dt>Side</dt>
			<dd>
								<img src="/cardlist/partimages/w.gif" alt="" decoding="async">
								</dd>
			</dl>
			<dl>
			<dt>Color</dt>
			<dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Level</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Cost</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Power</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Trigger</dt>
			<dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
			</dl>
			<dl>
			<dt>Soul</dt>
			<dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail u-mt-22 u-mt-40-sp">
			<p>Climax effect.</p>
		</div>
		</div>
	</div>
</div>
`
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatal(err)
		}
		card := extractData(siteConfigs[English], doc.Clone(), nil)
		if card.Type != CardTypeClimax {
			t.Fatalf("Type: got %q want CX", card.Type)
		}
		if !equalIntPtr(card.Soul, intPtr(1)) {
			t.Fatalf("Soul: got %v want 1", card.Soul)
		}
	})
}

func TestExtractData_soulFromMarkupWhenCardTypeNotCharacter_jp(t *testing.T) {
	html := `
	<th><a href="/cardlist/?cardno=XX/W99-001&amp;l"><img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/x/x_test/x_test_001.gif" alt="イベントなのにソウル"></a></th>
	<td>
	<h4><a href="/cardlist/?cardno=XX/W99-001&amp;l"><span class="highlight_target">
	イベントなのにソウル</span>(<span class="highlight_target">XX/W99-001</span>)</a> -「テスト」Vol.1<br></h4>
	<span class="unit">
	サイド：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/w.gif"></span>
	<span class="unit">種類：イベント</span>
	<span class="unit">レベル：0</span><br>
	<span class="unit">色：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/yellow.gif"></span>
	<span class="unit">パワー：-</span>
	<span class="unit">ソウル：<img src="https://s3-ap-northeast-1.amazonaws.com/static.ws-tcg.com/wordpress/wp-content/cardimages/_partimages/soul.gif"></span>
	<span class="unit">コスト：0</span><br>
	<span class="unit">レアリティ：C</span>
	<span class="unit">トリガー：－</span>
	<span class="unit">特徴：<span class="highlight_target">-・-</span></span><br>
	<span class="unit">フレーバー：-</span><br>
	<br>
	<span class="highlight_target">テスト</span>
	</td>
	`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	card := extractData(siteConfigs[Japanese], doc.Clone(), nil)
	if card.Type != CardTypeEvent {
		t.Fatalf("Type: got %q want EV", card.Type)
	}
	if !equalIntPtr(card.Soul, intPtr(1)) {
		t.Fatalf("Soul: got %v want 1", card.Soul)
	}
}

func TestExtractDataCX_en(t *testing.T) {
	climax := `
<div class="p-cards__detail-wrapper">
	<div class="p-cards__detail-wrapper-inner">
		<div class="image"><img src="/wp/wp-content/images/cardimages/SS/WE41_E59SHP.png" alt="Direct Confrontation!" decoding="async">
		</div>
		<div class="p-cards__detail-textarea">
		<p class="number">SS/WE41-E59SHP</p>
		<p class="ttl u-mt-14 u-mt-16-sp">Direct Confrontation!</p>
		<div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Expansion</dt>
			<dd>[EX] Shakugan no Shana</dd>
			</dl>
			<dl>
			<dt>Traits</dt>
			<dd></dd>
			</dl>
			<dl>
			<dt>Card Type</dt>
			<dd>Climax</dd>
			</dl>
			<dl>
			<dt>Rarity</dt>
			<dd>SHP</dd>
			</dl>
			<dl>
			<dt>Side</dt>
			<dd>
								<img src="/cardlist/partimages/w.gif" alt="" decoding="async">
								</dd>
			</dl>
			<dl>
			<dt>Color</dt>
			<dd><img src="/wp/wp-content/images/partimages/blue.gif"></dd>
			</dl>
		</div>
		<div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
			<dl>
			<dt>Level</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Cost</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Power</dt>
			<dd>-</dd>
			</dl>
			<dl>
			<dt>Trigger</dt>
			<dd><img src="/wp/wp-content/images/partimages/soul.gif"><img src="/wp/wp-content/images/partimages/gate.gif"></dd>
			</dl>
			<dl>
			<dt>Soul</dt>
			<dd>-</dd>
			</dl>
		</div>
		<div class="p-cards__detail u-mt-22 u-mt-40-sp">
			<p>【CONT】 All of your characters get +1000 power and +1 soul.<br>(<img src="/wp/wp-content/images/partimages/gate.gif">: When this card triggers, you may choose 1 climax in your waiting room, and return it to your hand)<br></p>
		</div>
		<div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
			<p>Flow inside, O energy.</p>
		</div>
		<p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">© YASHICHIRO TAKAHASHI/NOIZI ITO/ASCII MEDIA WORKS/「SHAKUGAN NO ShanaⅡ」COMMITTEE/MBS</p>
		</div>
	</div>
</div>
`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(climax))
	if err != nil {
		t.Fatal(err)
	}

	card := extractData(siteConfigs[English], doc.Clone(), nil)

	if card.Type != "CX" {
		t.Errorf("got %v: expected CX", card.Type)
	}

	if card.Name != "Direct Confrontation!" {
		t.Errorf("got %v: expected Direction Confrontation!", card.Name)
	}

	if card.Color != "BLUE" {
		t.Errorf("got %v: expected BLUE", card.Color)
	}

	if card.Soul != nil {
		t.Errorf("got %v: expected nil", card.Soul)
	}

	if card.Level != nil {
		t.Errorf("got %v: expected nil", card.Level)
	}

	if card.Cost != nil {
		t.Errorf("got %v: expected nil", card.Cost)
	}

	expectedTrigger := []Trigger{TriggerSoul, TriggerGate}
	if !equalSlice(card.Triggers, expectedTrigger) {
		t.Errorf("got %v: expected %v", card.Triggers, expectedTrigger)
	}

	expectedAbility := []string{
		"【CONT】 All of your characters get +1000 power and +1 soul.",
		"([GATE]: When this card triggers, you may choose 1 climax in your waiting room, and return it to your hand)",
	}
	if !equalSlice(card.Text, expectedAbility) {
		t.Errorf("Incorrect ability. Got %v, want %v", card.Text, expectedAbility)
	}
}

func TestExtractData_en_specialCardNumbers(t *testing.T) {
	testcases := []struct {
		name         string
		html         string
		lang         SiteLanguage
		expectedCard Card
	}{
		{
			`"A Nice Change" Kanon Matsubara`,
			`<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/b/bd_en_w03/BD_EN_W03_004.png" alt="&quot;A Nice Change&quot; Kanon Matsubara" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">BD/EN-W03-004</p>
            <p class="ttl u-mt-14 u-mt-16-sp">"A Nice Change" Kanon Matsubara</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>BanG Dream! Girls Band Party! MULTI LIVE</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Music・Hello, Happy World!</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>R</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/w.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/yellow.gif"></dd>
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
                <dd>1000</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】At the beginning of your climax phase, choose 1 of your 《Music》 characters, and that character gets +1000 power until end of turn.<br>【ACT】Brainstorm [(1)【REST】this card] Flip over 4 cards from the top of your deck, and put it into your waiting room. For each climax revealed among those cards, draw up to 1 card.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>All it takes is something small for people to change the way we think and act... That's all it took for us.</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©BanG Dream! Project ©Craft Egg Inc. ©bushiroad All Rights Reserved.</p>
          </div>
        </div>
      </div>`,
			English,
			Card{
				CardNumber:    "BD/EN-W03-004",
				SetID:         "BD",
				ExpansionName: "BanG Dream! Girls Band Party! MULTI LIVE",
				Sides:         []Side{SideWeiss},
				Release:       "EN-W03",
				ReleasePackID: "03",
				ID:            "004",
				Language:      "en",
				Type:          "CH",
				Name:          `"A Nice Change" Kanon Matsubara`,
				Color:         "YELLOW",
				Soul:          intPtr(1),
				Level:         intPtr(0),
				Cost:          intPtr(0),
				FlavorText:    "All it takes is something small for people to change the way we think and act... That's all it took for us.",
				Power:         intPtr(1000),
				Rarity:        "R",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/b/bd_en_w03/BD_EN_W03_004.png",
				Triggers:      []Trigger{},
				Traits:        []string{"Music", "Hello, Happy World!"},
				Text: []string{
					"【AUTO】At the beginning of your climax phase, choose 1 of your 《Music》 characters, and that character gets +1000 power until end of turn.",
					"【ACT】Brainstorm [(1)【REST】this card] Flip over 4 cards from the top of your deck, and put it into your waiting room. For each climax revealed among those cards, draw up to 1 card.",
				},
			},
		},
		{
			"Idol Theme Cup 2024",
			`<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/updates/PR/WS_TCPR_P01.png" alt="Idol Theme Cup 2024" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">WS/TCPR-P01</p>
            <p class="ttl u-mt-14 u-mt-16-sp">Idol Theme Cup 2024</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>PR Card 【Weiẞ Side】</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd></dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Climax</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>PR</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/w.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/red.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd>-</dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【CONT】  All of your characters get +2 soul.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>-</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">おきたくん</p>
          </div>
        </div>
      </div>`,
			English,
			Card{
				CardNumber:    "WS/TCPR-P01",
				SetID:         "WS",
				ExpansionName: "PR Card 【Weiẞ Side】",
				Sides:         []Side{SideWeiss},
				Release:       "TCPR",
				ReleasePackID: "",
				ID:            "P01",
				Language:      "en",
				Type:          "CX",
				Name:          "Idol Theme Cup 2024",
				Color:         "RED",
				Soul:          nil,
				Level:         nil,
				Cost:          nil,
				FlavorText:    "",
				Power:         nil,
				Rarity:        "PR",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/updates/PR/WS_TCPR_P01.png",
				Triggers:      []Trigger{TriggerSoul, TriggerSoul},
				Traits:        []string{},
				Text: []string{
					"【CONT】  All of your characters get +2 soul.",
				},
			},
		},
		{
			"Lie Ren",
			`<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/RWBY/RWBY_WX03_020PR.png" alt="Lie Ren" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">RWBY/BRO2021-01+PR</p>
            <p class="ttl u-mt-14 u-mt-16-sp">Lie Ren</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>PR Card 【Weiẞ Side】</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Remnant・JNPR</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>PR</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/w.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/green.gif"></dd>
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
                <dd>500</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 When this card becomes 【REVERSE】, if you have another 《Remnant》 character, and this card's battle opponent is level 0 or lower, you may put the top card of your opponent's clock into their waiting room. If you do, put that character into your opponent's clock.<br>【AUTO】 [(1)] When this card is put into your waiting room from the stage, you may pay the cost. If you do, look at up to 3 cards from the top of your deck, choose 1 card from among them, put it into your clock, and put the rest into your waiting room. If you put 1 card into your clock, choose 1 《Remnant》 character in your waiting room, and return it to your hand.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p></p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">© 2021 ROOSTER TEETH PRODUCTIONS, LLC, ALL RIGHTS RESERVED.</p>
          </div>
        </div>
      </div>`,
			English,
			Card{
				// The website puts the card number as "RWBY/BRO2021-01+PR",
				// but it's actually "RWBY/BRO2021-01 PR".
				CardNumber:    "RWBY/BRO2021-01 PR",
				SetID:         "RWBY",
				ExpansionName: "PR Card 【Weiẞ Side】",
				Sides:         []Side{SideWeiss},
				Release:       "BRO2021",
				ReleasePackID: "2021",
				ID:            "01 PR",
				Language:      "en",
				Type:          "CH",
				Name:          "Lie Ren",
				Color:         "GREEN",
				Soul:          intPtr(1),
				Level:         intPtr(0),
				Cost:          intPtr(0),
				FlavorText:    "",
				Power:         intPtr(500),
				Rarity:        "PR",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/RWBY/RWBY_WX03_020PR.png",
				Triggers:      []Trigger{},
				Traits:        []string{"Remnant", "JNPR"},
				Text: []string{
					"【AUTO】 When this card becomes 【REVERSE】, if you have another 《Remnant》 character, and this card's battle opponent is level 0 or lower, you may put the top card of your opponent's clock into their waiting room. If you do, put that character into your opponent's clock.",
					"【AUTO】 [(1)] When this card is put into your waiting room from the stage, you may pay the cost. If you do, look at up to 3 cards from the top of your deck, choose 1 card from among them, put it into your clock, and put the rest into your waiting room. If you put 1 card into your clock, choose 1 《Remnant》 character in your waiting room, and return it to your hand.",
				},
			},
		},
		{
			"Moment Between the Two, Sally",
			`<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/updates/PR/BFR_BSL2021_03SPR.png" alt="Moment Between the Two, Sally" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">BFR/BSL2021-03S</p>
            <p class="ttl u-mt-14 u-mt-16-sp">Moment Between the Two, Sally</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>PR Card 【Schwarz Side】</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Game・Weapon</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>PR</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/s.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/blue.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>1</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>0</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>4000</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 When your climax is placed on your climax area, this card gets +3000 power until end of turn.<br>【AUTO】 【CXCOMBO】 When this card attacks, if "Never-Ending Sunset Area" is in your climax area, and you have another 《Game》 character, put the top 2 cards of your deck into your waiting room, choose up to 1 level X or lower 《Game》 character in your waiting room, and return it to your hand. X is equal to the total level of the cards put into your waiting room by this effect. (Climax are regarded as level 0)</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>-</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©2020 Yuumikan・Koin/KADOKAWA/Bofuri Project</p>
          </div>
        </div>
      </div>`,
			English,
			Card{
				CardNumber:    "BFR/BSL2021-03S",
				SetID:         "BFR",
				ExpansionName: "PR Card 【Schwarz Side】",
				Sides:         []Side{SideSchwarz},
				Release:       "BSL2021",
				ReleasePackID: "2021",
				ID:            "03S",
				Language:      "en",
				Type:          "CH",
				Name:          "Moment Between the Two, Sally",
				Color:         "BLUE",
				Soul:          intPtr(1),
				Level:         intPtr(1),
				Cost:          intPtr(0),
				FlavorText:    "",
				Power:         intPtr(4000),
				Rarity:        "PR",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/updates/PR/BFR_BSL2021_03SPR.png",
				Triggers:      []Trigger{},
				Traits:        []string{"Game", "Weapon"},
				Text: []string{
					"【AUTO】 When your climax is placed on your climax area, this card gets +3000 power until end of turn.",
					"【AUTO】 【CXCOMBO】 When this card attacks, if \"Never-Ending Sunset Area\" is in your climax area, and you have another 《Game》 character, put the top 2 cards of your deck into your waiting room, choose up to 1 level X or lower 《Game》 character in your waiting room, and return it to your hand. X is equal to the total level of the cards put into your waiting room by this effect. (Climax are regarded as level 0)",
				},
			},
		},
		{
			"Triumphant Return, Rimuru",
			`<div class="p-cards__detail-wrapper">
        <div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/TSK2/TSK_S82_E070S.png" alt="Triumphant Return, Rimuru" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">TSK/S82-E070SSP%2B</p>
            <p class="ttl u-mt-14 u-mt-16-sp">Triumphant Return, Rimuru</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>That Time I Got Reincarnated as a Slime Vol.2 </dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Demon Continent・Slime</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>SSP+</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/s.gif" alt="" decoding="async">
                                  </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/blue.gif"></dd>
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
                <dd>2000</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd>-</dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 When this card is placed on the stage from your hand, reveal the top card of your deck. If that card is a 《Demon Continent》 character, this card gets +1 level and +1500 power until end of turn. (Return the revealed card to its original place)<br>【AUTO】 When this card's battle opponent becomes 【REVERSE】, choose 1 of your other 《Demon Continent》 characters, 【REST】 it, and move it to an open position of your back stage.</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>-</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">© Taiki Kawakami, Fuse, KODANSHA/“Ten-Sura” Project</p>
          </div>
        </div>
      </div>`,
			English,
			Card{
				CardNumber:    "TSK/S82-E070SSP+",
				SetID:         "TSK",
				ExpansionName: "That Time I Got Reincarnated as a Slime Vol.2",
				Sides:         []Side{SideSchwarz},
				Release:       "S82",
				ReleasePackID: "82",
				ID:            "E070SSP+",
				Language:      "en",
				Type:          "CH",
				Name:          "Triumphant Return, Rimuru",
				Color:         "BLUE",
				Soul:          intPtr(1),
				Level:         intPtr(0),
				Cost:          intPtr(0),
				FlavorText:    "",
				Power:         intPtr(2000),
				Rarity:        "SSP+",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/TSK2/TSK_S82_E070S.png",
				Triggers:      []Trigger{},
				Traits:        []string{"Demon Continent", "Slime"},
				Text: []string{
					"【AUTO】 When this card is placed on the stage from your hand, reveal the top card of your deck. If that card is a 《Demon Continent》 character, this card gets +1 level and +1500 power until end of turn. (Return the revealed card to its original place)",
					"【AUTO】 When this card's battle opponent becomes 【REVERSE】, choose 1 of your other 《Demon Continent》 characters, 【REST】 it, and move it to an open position of your back stage.",
				},
			},
		},
		{
			"To Stand Side by Side, Sayo Hikawa",
			`<div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/BDCC/WE42_E096_N.png" alt="To Stand Side by Side, Sayo Hikawa" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">BD/WE42_E096_N</p>
            <p class="ttl u-mt-14 u-mt-16-sp">To Stand Side by Side, Sayo Hikawa</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>[EX] Bang Dream! Girls Band Party! Countdown Collection</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Music・Roselia</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>N</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/w.gif" alt="" decoding="async">
                                                    </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd><img src="/wp/wp-content/images/partimages/blue.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>2</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>1</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>2500</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【AUTO】 [(2) Put 1 character from your stage into your waiting room] When you use this card's "Backup", you may pay the cost. If you do, choose 1 of your opponent's characters with level higher than your opponent's level, and put it into their waiting room.<br>【ACT】 【COUNTER】 Backup 2500, Level 2 [(1) Put this card from your hand into your waiting room] (Choose 1 of your characters that is being frontal attacked, and that character gets +2500 power until end of turn)</p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>―</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©BanG Dream! Project ©Craft Egg Inc. ©bushiroad All Rights Reserved. illust.かがちさく</p>
          </div>
        </div>`,
			English,
			Card{
				CardNumber:    "BD/WE42-E096 N",
				SetID:         "BD",
				ExpansionName: "[EX] Bang Dream! Girls Band Party! Countdown Collection",
				Sides:         []Side{SideWeiss},
				Release:       "WE42",
				ReleasePackID: "42",
				ID:            "E096 N",
				Language:      "en",
				Type:          "CH",
				Name:          "To Stand Side by Side, Sayo Hikawa",
				Color:         "BLUE",
				Soul:          intPtr(1),
				Level:         intPtr(2),
				Cost:          intPtr(1),
				FlavorText:    "",
				Power:         intPtr(2500),
				Rarity:        "N",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/BDCC/WE42_E096_N.png",
				Triggers:      []Trigger{TriggerSoul},
				Traits:        []string{"Music", "Roselia"},
				Text: []string{
					"【AUTO】 [(2) Put 1 character from your stage into your waiting room] When you use this card's \"Backup\", you may pay the cost. If you do, choose 1 of your opponent's characters with level higher than your opponent's level, and put it into their waiting room.",
					"【ACT】 【COUNTER】 Backup 2500, Level 2 [(1) Put this card from your hand into your waiting room] (Choose 1 of your characters that is being frontal attacked, and that character gets +2500 power until end of turn)",
				},
			},
		},
	}

	for _, tc := range testcases {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
		if err != nil {
			t.Error(err)
			continue
		}

		card := extractData(siteConfigs[tc.lang], doc.Clone(), nil)
		assertCardEqualsWithTitle(t, tc.name, card, tc.expectedCard)
	}
}

func TestExtractData_en_improperColor(t *testing.T) {
	testcases := []struct {
		name         string
		html         string
		lang         SiteLanguage
		expectedCard Card
	}{
		{
			`"Fake Priest?" Heiter`,
			`<div class="p-cards__detail-wrapper-inner">
          <div class="image"><img src="/wp/wp-content/images/cardimages/SFN/S108_E020.png" alt="&quot;Fake Priest?&quot; Heiter" decoding="async">
          </div>
          <div class="p-cards__detail-textarea">
            <p class="number">SFN/S108-E020</p>
            <p class="ttl u-mt-14 u-mt-16-sp">"Fake Priest?" Heiter</p>
            <div class="p-cards__detail-type u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Expansion</dt>
                <dd>Frieren: Beyond Journey’s End</dd>
              </dl>
              <dl>
                <dt>Traits</dt>
                <dd>Adventurer・Magic</dd>
              </dl>
              <dl>
                <dt>Card Type</dt>
                <dd>Character</dd>
              </dl>
              <dl>
                <dt>Rarity</dt>
                <dd>C</dd>
              </dl>
              <dl>
                <dt>Side</dt>
                <dd>
                                    <img src="/cardlist/partimages/s.gif" alt="" decoding="async">
                                                    </dd>
              </dl>
              <dl>
                <dt>Color</dt>
                <dd>[[yellow.gif]]</dd>
              </dl>
            </div>
            <div class="p-cards__detail-status u-mt-22 u-mt-40-sp">
              <dl>
                <dt>Level</dt>
                <dd>2</dd>
              </dl>
              <dl>
                <dt>Cost</dt>
                <dd>1</dd>
              </dl>
              <dl>
                <dt>Power</dt>
                <dd>4500</dd>
              </dl>
              <dl>
                <dt>Trigger</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
              <dl>
                <dt>Soul</dt>
                <dd><img src="/wp/wp-content/images/partimages/soul.gif"></dd>
              </dl>
            </div>
            <div class="p-cards__detail u-mt-22 u-mt-40-sp">
              <p>【CONT】 Assist All of your characters in front of this card get +X power. X is equal to that character's level ×500.<br>【ACT】 [(2) 【REST】 this card] Put the top card of your clock into your waiting room.<br></p>
            </div>
            <div class="p-cards__detail-serif u-mt-22 u-mt-40-sp">
              <p>Himmel: "That brat who said that to me is now a fake priest who just drinks all the time."</p>
            </div>
            <p class="p-cards__detail-copyrights u-mt-22 u-mt-40-sp">©Kanehito Yamada, Tsukasa Abe/Shogakukan/ “Frieren”Project</p>
          </div>
        </div>`,
			English,
			Card{
				CardNumber:    "SFN/S108-E020",
				SetID:         "SFN",
				ExpansionName: "Frieren: Beyond Journey’s End",
				Sides:         []Side{SideSchwarz},
				Release:       "S108",
				ReleasePackID: "108",
				ID:            "E020",
				Language:      "en",
				Type:          "CH",
				Name:          `"Fake Priest?" Heiter`,
				Color:         "YELLOW",
				Soul:          intPtr(1),
				Level:         intPtr(2),
				Cost:          intPtr(1),
				FlavorText:    `Himmel: "That brat who said that to me is now a fake priest who just drinks all the time."`,
				Power:         intPtr(4500),
				Rarity:        "C",
				ImageURL:      "https://en.ws-tcg.com/wp/wp-content/images/cardimages/SFN/S108_E020.png",
				Triggers:      []Trigger{TriggerSoul},
				Traits:        []string{"Adventurer", "Magic"},
				Text: []string{
					"【CONT】 Assist All of your characters in front of this card get +X power. X is equal to that character's level ×500.",
					"【ACT】 [(2) 【REST】 this card] Put the top card of your clock into your waiting room.",
				},
			},
		},
	}

	for _, tc := range testcases {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(tc.html))
		if err != nil {
			t.Error(err)
			continue
		}

		card := extractData(siteConfigs[tc.lang], doc.Clone(), nil)
		assertCardEqualsWithTitle(t, tc.name, card, tc.expectedCard)
	}
}
