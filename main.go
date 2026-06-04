package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error: cannot load .env")
	}

	pageURL := os.Getenv("DOWNLOADED_WEBPAGE_URL")

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var html string

	err = chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady(`[data-marker="item"][data-item-id]`, chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(html[:100])

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Fatal(err)
	}

	baseURL, err := url.Parse(pageURL)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(`[data-marker="item"][data-item-id]`).Each(func(_ int, card *goquery.Selection) {
		id, _ := card.Attr("data-item-id")
		fmt.Println(id)

		titleEl := card.Find(`[data-marker="item-title"]`).First()
		title := cleanText(titleEl.Text())
		fmt.Println(title)

		href, _ := titleEl.Attr("href")
		URL := cleanURL(baseURL, href)
		fmt.Println(URL)

		priceRaw, _ := card.Find(`[data-marker="item-price"] meta[itemprop="price"]`).First().Attr("content")
		//fmt.Println(priceRaw)
		parsedPrice := parsePrice(priceRaw)
		fmt.Println(*parsedPrice)
		currency, _ := card.Find(`[data-marker="item-price"] meta[itemprop="priceCurrency"]`).First().Attr("content")
		fmt.Println(currency)
		description, _ := card.Find(`meta[itemprop="description"]`).First().Attr("content")
		fmt.Println(description)
		// image, _ := card.Find(`[data-marker="item-image"] img[itemprop="image"]`).First().Attr("src")
		// fmt.Println(image) // тут видимо куски ссылок

		// images := []string{}
		// card.Find(`[data-marker^="slider-image/image-"]`).Each(func(_ int, imgNode *goquery.Selection) {
		// 	marker, ok := imgNode.Attr("data-marker")
		// 	if ok {
		// 		images = append(images, strings.TrimPrefix(marker, "slider-image/image-"))
		// 	}
		// })

		PriceText := cleanText(card.Find(`[data-marker="item-price-value"]`).First().Text())
		fmt.Println(PriceText) // это можно потом убрать - или использовать для вывода потом

		badges := []string{}
		card.Find(`[data-marker^="badge-title-"]`).Each(func(_ int, badge *goquery.Selection) {
			if text := cleanText(badge.Text()); text != "" {
				badges = append(badges, text)
			}
		})

		Location := cleanText(card.Find(`[data-marker="item-location"]`).First().Text())
		fmt.Println(Location)

		// Date := cleanText(card.Find(`[data-marker="item-date"]`).First().Text())
		// fmt.Println(Date) //может посикать ещё, но пока относительная дата объявления не очень полезна

		Rating := cleanText(card.Find(`[data-marker="seller-rating/score"]`).First().Text())
		fmt.Println(Rating)
		Reviews := cleanText(card.Find(`[data-marker="seller-info/summary"]`).First().Text())
		fmt.Println(Reviews)

		fmt.Println(badges)

		HasMessenger := card.Find(`[data-marker="messenger-button"], [data-marker="messenger-button/link"]`).Length() > 0
		fmt.Println(HasMessenger)

		HasPhoneButton := card.Find(`[data-marker^="item-phone-button"]`).Length() > 0
		fmt.Println(HasPhoneButton)

		fmt.Println()
	})

}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func cleanURL(base *url.URL, href string) string {
	if href == "" {
		return ""
	}

	u, err := url.Parse(href)
	if err != nil {
		return href
	}

	absolute := base.ResolveReference(u)
	query := absolute.Query()
	query.Del("context")
	//absolute.RawQuery = query.Encode()
	// delete excessive parameters
	absolute.RawQuery = ""

	return absolute.String()
}

func parsePrice(s string) *int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}
