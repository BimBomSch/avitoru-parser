package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
)

type AvitoAdvert struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	Price          *int   `json:"price"`
	Currency       string `json:"currency"`
	PriceText      string `json:"priceText"`
	Description    string `json:"description"`
	Location       string `json:"location"`
	Rating         string `json:"rating"`
	Reviews        string `json:"reviews"`
	HasMessenger   bool   `json:"hasMessenger"`
	HasPhoneButton bool   `json:"hasPhoneButton"`
}

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

	adverts, err := ParseAvitoAdverts(html, pageURL)
	if err != nil {
		log.Fatal(err)
	}

	// out, _ := json.MarshalIndent(adverts, "", "  ")
	// fmt.Println(string(out))
	// fmt.Println("adverts:", len(adverts))

	//tagsFromValue := GetJSONTags(adverts[0])
	// fmt.Println("Field names(from json-tags):", tagsFromValue)

	// fmt.Println(adverts[0])
	// slice := StructToStringSlice(adverts[0])
	// fmt.Printf("%q\n", slice)

	advertsCSV := make([][]string, 0, len(adverts)+1)
	advertsCSV = append(advertsCSV, GetJSONTags(adverts[0]))
	for _, advert := range adverts {
		advertsCSV = append(advertsCSV, StructToStringSlice(advert))
	}
	//fmt.Println(advertsCSV)

	file, err := os.Create("output.csv")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer file.Close()

	file.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM signature, to show correct Russian in Excel

	writer := csv.NewWriter(file)
	writer.Comma = ';' // for correct csv parsing by Excel
	for _, record := range advertsCSV {
		err := writer.Write(record)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}

	writer.Flush()
	//fmt.Println("CSV written to folder!")
}

func ParseAvitoAdverts(html string, pageURL string) ([]AvitoAdvert, error) {
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var adverts []AvitoAdvert

	doc.Find(`[data-marker="item"][data-item-id]`).Each(func(_ int, advert *goquery.Selection) {
		id, _ := advert.Attr("data-item-id")

		titleEl := advert.Find(`[data-marker="item-title"]`).First()
		href, _ := titleEl.Attr("href")

		priceRaw, _ := advert.Find(`[data-marker="item-price"] meta[itemprop="price"]`).First().Attr("content")
		currency, _ := advert.Find(`[data-marker="item-price"] meta[itemprop="priceCurrency"]`).First().Attr("content")
		description, _ := advert.Find(`meta[itemprop="description"]`).First().Attr("content")

		adverts = append(adverts, AvitoAdvert{
			ID:             id,
			Title:          cleanText(titleEl.Text()),
			URL:            cleanURL(baseURL, href),
			Price:          parsePrice(priceRaw),
			Currency:       currency,
			PriceText:      cleanText(advert.Find(`[data-marker="item-price-value"]`).First().Text()),
			Description:    cleanText(description),
			Location:       cleanText(advert.Find(`[data-marker="item-location"]`).First().Text()),
			Rating:         cleanText(advert.Find(`[data-marker="seller-rating/score"]`).First().Text()),
			Reviews:        cleanText(advert.Find(`[data-marker="seller-info/summary"]`).First().Text()),
			HasMessenger:   advert.Find(`[data-marker="messenger-button"], [data-marker="messenger-button/link"]`).Length() > 0,
			HasPhoneButton: advert.Find(`[data-marker^="item-phone-button"]`).Length() > 0,
		})
	})

	return adverts, nil
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

func GetJSONTags(v any) []string {
	t := reflect.TypeOf(v)

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	var tags []string

	for field := range t.Fields() {
		tagValue := field.Tag.Get("json")

		if tagValue == "-" || tagValue == "" {
			continue
		}

		cleanTag, _, _ := strings.Cut(tagValue, ",")
		tags = append(tags, cleanTag)
	}

	return tags
}

func StructToStringSlice(obj any) []string {
	val := reflect.ValueOf(obj)

	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	result := make([]string, 0, val.NumField())

	for _, field := range val.Fields() {

		if field.Kind() == reflect.Pointer && !field.IsNil() {
			field = field.Elem()
		}

		strVal := fmt.Sprint(field.Interface())
		result = append(result, strVal)
	}

	return result
}
