package cmd

import (
	"bytes"
	"path"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var baseRarity = [6]string{
	"RR",
	"R",
	"U",
	"C",
	"CR",
	"CC",
}

var triggersMap = map[string]string{
	"soul":     "SOUL",
	"salvage":  "COMEBACK",
	"draw":     "DRAW",
	"stock":    "POOL",
	"treasure": "TREASURE",
	"shot":     "SHOT",
	"bounce":   "RETURN",
	"gate":     "GATE",
	"standby":  "STANDBY",
}

func parseInt(st string) string {
	res := strings.Split(st, "：")[1]
	if strings.Contains(res, "-") {
		res = "0"
	}
	return res
}

// ExtractData extract data to card
func ExtractData(mainHtml *goquery.Selection) Card {
	trigger := []string{}
	sa := []string{}
	complex := mainHtml.Find("h4 span").Last().Text()
	set := strings.Split(complex, "/")[0]
	side := strings.Split(complex, "/")[1][0]
	setInfo := strings.Split(strings.Split(complex, "/")[1][1:], "-")
	var ability, _ = mainHtml.Find("span").Last().Html()
	setName := strings.TrimSpace(strings.Split(mainHtml.Find("h4").Text(), ") -")[1])

	infos := mainHtml.Find(".unit").Map(func(i int, s *goquery.Selection) string {
		if s.Text() == "色：" {
			_, colorName := path.Split(s.Children().AttrOr("src", "yay"))
			return strings.ToUpper(strings.Split(colorName, ".")[0])
		}
		if strings.HasPrefix(s.Text(), "種類：") {
			var cType = strings.TrimSpace(strings.Split(s.Text(), "種類：")[1])

			switch cType {
			case "イベント":
				return "EV"
			case "キャラ":
				return "CH"
			case "クライマックス":
				return "CX"
			}
		}
		if strings.HasPrefix(s.Text(), "ソウル：") {
			return strconv.Itoa(s.Children().Length())
		}
		if strings.HasPrefix(s.Text(), "トリガー：") {
			var res bytes.Buffer
			s.Children().Each(func(i int, ss *goquery.Selection) {
				if i != 0 {
					res.WriteString(" ")
				}
				_, trigger := path.Split(ss.AttrOr("src", "yay"))
				res.WriteString(triggersMap[strings.Split(trigger, ".")[0]])
			})
			return strings.ToUpper(res.String())
		}
		if strings.HasPrefix(s.Text(), "特徴：") {
			var res bytes.Buffer
			s.Children().Each(func(i int, ss *goquery.Selection) {
				res.WriteString(ss.Text())
			})
			if strings.Contains(res.String(), "-") {
				return ""
			}
			return res.String()
		}
		return s.Text()
	})

	if infos[8] != "" {
		trigger = strings.Split(infos[8], " ")
	}

	if infos[9] != "" {
		sa = strings.Split(infos[9], "・")
	}
	card := Card{
		JpName:        strings.TrimSpace(mainHtml.Find("h4 span").First().Text()),
		Set:           set,
		SetName:       setName,
		Side:          string(side),
		Release:       setInfo[0],
		ID:            setInfo[1],
		CardType:      infos[1],
		Level:         parseInt(infos[2]),
		Colour:        infos[3],
		Power:         parseInt(infos[4]),
		Soul:          infos[5],
		Cost:          parseInt(infos[6]),
		Rarity:        strings.Split(infos[7], "：")[1],
		Trigger:       trigger,
		SpecialAttrib: sa,
		Ability:       strings.Split(ability, "<br/>"),
	}
	return card

}

func IsbaseRarity(card Card) bool {

	for _, rarity := range baseRarity {
		if rarity == card.Rarity {
			return true
		}
	}
	return false
}
