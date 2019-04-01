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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"

	"golang.org/x/net/publicsuffix"

	"github.com/PuerkitoBio/goquery"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var serieNumber string
var allRarity bool

const baseurl = "https://ws-tcg.com/cardlist/search"

// Card info to export
type Card struct {
	Set               string   `json:"set"`
	SetName           string   `json:"setName"`
	Side              string   `json:"side"`
	Release           string   `json:"release"`
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	JpName            string   `json:"jpName"`
	CardType          string   `json:"cardType"`
	Colour            string   `json:"colour"`
	Level             string   `json:"level"`
	Cost              string   `json:"cost"`
	Power             string   `json:"power"`
	Soul              string   `json:"soul"`
	Rarity            string   `json:"rarity"`
	BreakDeckbuilding bool     `json:"breakDeckbuilding"`
	ENEquivalent      bool     `json:"EN_Equivalent"`
	FlavourText       string   `json:"flavourText"`
	Trigger           []string `json:"trigger"`
	Ability           []string `json:"ability"`
	SpecialAttrib     []string `json:"specialAttrib"`
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wsoffcli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			log.Fatal(err)
		}
		page := 1
		client := &http.Client{Jar: jar}
		values := url.Values{
			"cmd":             {"search"},
			"show_page_count": {"100"},
		}
		if serieNumber != "" {
			values.Add("expansion", serieNumber)
		}
		for {
			resp, err := client.PostForm(fmt.Sprintf("%v?page=%d", baseurl, page), values)
			if err != nil {
				log.Fatal(err)
			}
			if resp.StatusCode == 404 {
				break
			}
			log.Println("Fetch page : ", page, "with params : ", values)
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.Fatal(err)
			}
			doc.Find(".search-result-table tr").Each(func(i int, s *goquery.Selection) {
				var buffer bytes.Buffer
				card := ExtractData(s)

				if !allRarity {
					if !IsbaseRarity(card) {
						return
					}
				}
				res, errMarshal := json.Marshal(card)
				if errMarshal != nil {
					log.Println(errMarshal)
				}
				// fmt.Println(string(res))
				out, err := os.Create(fmt.Sprintf("%v-%v%v-%v.json", card.Set, card.Side, card.Release, card.ID))
				if err != nil {
					log.Println(err.Error())
				}
				defer out.Close()
				json.Indent(&buffer, res, "", "\t")
				buffer.WriteTo(out)
			})
			page = page + 1

		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&serieNumber, "serie", "", "serie number")
	rootCmd.PersistentFlags().BoolVarP(&allRarity, "allrarity", "a", false, "get all rarity (sp, ssp, sbr, etc...)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".wsoffcli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".wsoffcli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
