// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kwadkore/ws-scraper/fetch"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/text/language"
)

const maxWorker int = 5

func writeCards(wg *sync.WaitGroup, lang language.Tag, cardCh <-chan fetch.Card) {
	for card := range cardCh {
		res, errMarshal := json.Marshal(card)
		if errMarshal != nil {
			slog.Error(fmt.Sprintf("error marshalling: %v", errMarshal))
			continue
		}
		var buffer bytes.Buffer
		cardName := fmt.Sprintf("%v-%v-%v.json", card.SetID, card.Release, card.ID)
		dirName := filepath.Join(viper.GetString("cardDir"), lang.String(), card.SetID, card.Release)
		os.MkdirAll(dirName, 0o744)
		out, err := os.Create(filepath.Join(dirName, cardName))
		if err != nil {
			slog.Error(fmt.Sprintf("Error writing card: %v", err))
			continue
		}
		json.Indent(&buffer, res, "", "\t")
		buffer.WriteTo(out)
		out.Close()
		slog.Info(fmt.Sprintf("Finished card: %v", cardName))
	}
	wg.Done()
}

func writeBoosters(lang language.Tag, boosters map[string]fetch.Booster) {
	for k, v := range boosters {
		slog.Info(fmt.Sprintf("Writing booster: %v", k))
		dirName := filepath.Join(viper.GetString("boosterDir"), lang.String())
		os.MkdirAll(dirName, 0o744)
		filename := filepath.Join(dirName, k+".json")
		updatedData, err := json.Marshal(v.Cards)
		if err != nil {
			slog.Error("Error marshalling booster", "release", k, "error", err)
		}
		if err := os.WriteFile(filename, updatedData, 0o644); err != nil {
			slog.Error(fmt.Sprintf("Error writing booster: %v", k))
		}
	}
}

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch cards",
	Long: `Fetch cards

Use global switches to specify the set, by default it will fetch all sets.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := fetch.Config{
			GetAllRarities: viper.GetBool("allrarity"),
			GetRecent:      viper.GetBool("recent"),
			PageStart:      viper.GetInt("pagestart"),
			Reverse:        viper.GetBool("reverse"),
		}
		lang, err := language.Parse(viper.GetString("lang"))
		if err != nil {
			panic(fmt.Errorf("invalid language parameter: %v", err))
		}

		lBase, conf := language.Tag(lang).Base()
		if conf == language.No {
			panic(fmt.Errorf("completely unknown language: %v", cfg.Language))
		} else if conf != language.Exact {
			slog.Info(fmt.Sprintf("Checking base language %v with confidence %v", lBase, conf))
		}
		switch lBase.String() {
		case language.English.String():
			cfg.Language = fetch.English
		case language.Japanese.String():
			cfg.Language = fetch.Japanese
		default:
			panic(fmt.Sprintf("Unsupported language: %v", lang))
		}
		if serieNumber != "" {
			if s, err := strconv.Atoi(serieNumber); err == nil {
				cfg.ExpansionNumber = s
			} else {
				panic(fmt.Sprintf("Invalid expansion number: %v", err))
			}
		}
		if titleNumber != "" {
			if t, err := strconv.Atoi(titleNumber); err == nil {
				cfg.TitleNumber = t
			} else {
				panic(fmt.Sprintf("Invalid title number: %v", err))
			}
		}
		if neo != "" {
			cfg.SetCode = strings.Split(neo, "##")
		}

		slog.Debug("fetch called", "settings", viper.AllSettings())
		client, err := newFetchClient()
		if err != nil {
			panic(fmt.Errorf("invalid scraper client settings: %v", err))
		}
		defer client.Close()

		mode := viper.GetString("export")
		slog.Info(fmt.Sprintf("Start write in mode: %v", mode))
		switch mode {
		case "booster":
			bm, err := client.Boosters(context.Background(), cfg)
			if err != nil {
				slog.Error(fmt.Sprintf("Error fetching boosters: %v", err))
			}
			writeBoosters(lang, bm)
		case "card":
			cardCh := make(chan fetch.Card, maxWorker)
			var wg sync.WaitGroup
			for i := 0; i < maxWorker; i++ {
				wg.Add(1)
				go writeCards(&wg, lang, cardCh)
			}
			err := client.CardsStream(context.Background(), cfg, cardCh)
			if err != nil {
				slog.Error(fmt.Sprintf("Error fetching cards: %v", err))
			}
			wg.Wait()
		case "expansionlist":
			eMap, err := client.ExpansionList(context.Background(), cfg)
			if err != nil {
				slog.Error(fmt.Sprintf("Error fetching expansion list: %v", err))
			}
			if len(eMap) > 0 {
				var expansions []int
				for e := range eMap {
					expansions = append(expansions, e)
				}
				sort.Ints(expansions)
				fmt.Println("Expansions:")
				for _, e := range expansions {

					fmt.Printf("\t%d: %s\n", e, eMap[e])
				}
			}
		default:
			panic(fmt.Sprintf("Unsupported export mode: %q", mode))
		}
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// fetchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// fetchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	fetchCmd.Flags().StringP("boosterDir", "", "boosters", "Directory to put fetched booster information into")
	fetchCmd.Flags().StringP("cardDir", "", "cards", "Directory to put fetched card information into")
	fetchCmd.Flags().IntP("pagestart", "p", 0, "Start scanning from page #. Skip everything else before this page")
	fetchCmd.Flags().BoolP("reverse", "r", false, "Reverse order")
	fetchCmd.Flags().BoolP("allrarity", "a", false, "get all rarity (sp, ssp, sbr, etc...)")
	fetchCmd.Flags().StringP("export", "e", "card", "export value: card, booster, expansionlist, all")
	fetchCmd.Flags().String("lang", "ja", "Site language to pull from. Options are en or ja.")
	fetchCmd.Flags().BoolP("recent", "", false, "get all recent products")

	viper.BindPFlag("boosterDir", fetchCmd.Flags().Lookup("boosterDir"))
	viper.BindPFlag("cardDir", fetchCmd.Flags().Lookup("cardDir"))
	viper.BindPFlag("pagestart", fetchCmd.Flags().Lookup("pagestart"))
	viper.BindPFlag("reverse", fetchCmd.Flags().Lookup("reverse"))
	viper.BindPFlag("allrarity", fetchCmd.Flags().Lookup("allrarity"))
	viper.BindPFlag("export", fetchCmd.Flags().Lookup("export"))
	viper.BindPFlag("lang", fetchCmd.Flags().Lookup("lang"))
	viper.BindPFlag("recent", fetchCmd.Flags().Lookup("recent"))
}
