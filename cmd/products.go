/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/kwadkore/ws-scraper/fetch"
	"github.com/spf13/cobra"
)

func writeProducts(productList []fetch.ProductInfo) {
	res, errMarshal := json.Marshal(productList)
	if errMarshal != nil {
		slog.Error(fmt.Sprintf("Error marshalling: %v", errMarshal))
	}
	var buffer bytes.Buffer
	out, err := os.Create("product.json")
	if err != nil {
		slog.Error(fmt.Sprintf("Error writing: %v", err))
	}
	json.Indent(&buffer, res, "", "\t")
	buffer.WriteTo(out)
	out.Close()
	slog.Debug("Finished writing")
}

// productsCmd represents the products command
var productsCmd = &cobra.Command{
	Use:   "products",
	Short: "Get products information",
	Long: `Get products information.
It will output the ReleaseDate, Title, Image, SetCode, LicenceCode in a 'product.json' file.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("products called")

		client, err := newFetchClient()
		if err != nil {
			slog.Error(fmt.Sprintf("Error creating scraper client: %v", err))
			return
		}
		defer client.Close()

		productList, err := client.Products(context.Background(), cmd.Flag("page").Value.String())
		if err != nil {
			slog.Error(fmt.Sprintf("Error fetching products: %v", err))
		}
		writeProducts(productList)
	},
}

func init() {
	rootCmd.AddCommand(productsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// productsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	productsCmd.Flags().Int16P("page", "p", 1, "Give which page to parse")
}
