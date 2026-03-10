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
	"context"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const ProductsURL = "https://ws-tcg.com/products/page/"

var banProduct = []string{
	"new_title_ws",
	"resale_news",
	"bp_renewal",
}

var titleAndWorkNumberRegexp = regexp.MustCompile(`.*/ .*：([\w,]+)`)

// ProductInfo represents the extracted information from the HTML
type ProductInfo struct {
	ReleaseDate string
	Title       string
	LicenceCode string
	Image       string
	SetCode     string
}

func (c *Client) getDocument(ctx context.Context, rawURL string, referer string) (*goquery.Document, error) {
	resp, err := c.request(ctx, requestOptions{
		Method:  "GET",
		URL:     rawURL,
		Referer: referer,
	})
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(bytes.NewReader(resp.Body))
}

func extractProductInfo(doc *goquery.Document) (ProductInfo, error) {
	var setCode string
	releaseDate := strings.Split(strings.TrimSpace(doc.Find(".release strong").Text()), "(")[0]
	titleAndWorkNumber := strings.TrimSpace(doc.Find(".release").Text())

	matches := titleAndWorkNumberRegexp.FindStringSubmatch(titleAndWorkNumber)
	if matches == nil {
		return ProductInfo{}, fmt.Errorf("string %q doesn't match expected format", titleAndWorkNumber)
	}
	licenceCode := matches[1]
	doc.Find(".entry-content img").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		filename := path.Base(src)
		parts := strings.Split(filename, "_")
		if len(parts) >= 4 {
			setCode = parts[2]
		}
	})

	return ProductInfo{
		ReleaseDate: releaseDate,
		Title:       doc.Find(".entry-content > h3").Text(),
		LicenceCode: licenceCode,
		SetCode:     setCode,
		Image:       doc.Find(".product-detail .alignright img").AttrOr("src", "notfound"),
	}, nil
}

func (c *Client) Products(ctx context.Context, page string) ([]ProductInfo, error) {
	doc, err := c.getDocument(ctx, ProductsURL+page, "")
	if err != nil {
		return nil, err
	}

	var productList []ProductInfo
	var firstErr error
	doc.Find(".product-list .show-detail a").Each(func(i int, s *goquery.Selection) {
		productDetail := s.AttrOr("href", "")
		if productDetail == "" {
			return
		}
		for _, ban := range banProduct {
			if strings.Contains(productDetail, ban) {
				return
			}
		}

		slog.Info(fmt.Sprintf("Extract: %v", productDetail))
		productDoc, err := c.getDocument(ctx, productDetail, ProductsURL+page)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Error("Error fetching product detail", "url", productDetail, "error", err)
			return
		}

		productInfo, err := extractProductInfo(productDoc)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			slog.Error("Error getting product info", "url", productDetail, "error", err)
			return
		}
		productList = append(productList, productInfo)
	})

	return productList, firstErr
}
