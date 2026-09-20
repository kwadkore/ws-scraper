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
It will output the ReleaseDate, Title, Image, SetCode, LicenceCode in a 'product.json' file.

Whatever was fetched is written even when the request fails, but the
command then exits non-zero so scripts don't mistake a partial export for
a complete one.`,
	// A failed fetch is not a usage error; don't print the flag list for it.
	SilenceUsage: true,
	// The error is already logged where it happens; don't print it twice.
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("products called")

		client, err := newFetchClient()
		if err != nil {
			slog.Error(fmt.Sprintf("Error creating scraper client: %v", err))
			return err
		}
		defer client.Close()

		productList, fetchErr := client.Products(context.Background(), cmd.Flag("page").Value.String())
		if fetchErr != nil {
			slog.Error(fmt.Sprintf("Error fetching products: %v", fetchErr))
		}
		writeProducts(productList)
		return fetchErr
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
