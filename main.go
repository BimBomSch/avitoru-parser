package main

import (
	"bytes"
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
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
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
		chromedp.WaitReady(`[data-marker="item"][data-item-id]`, chromedp.ByQuery), // can comment for offline development.
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		log.Fatal(err)
	}

	adverts, err := ParseAvitoAdverts(html, pageURL)
	if err != nil {
		log.Fatal(err)
	}

	advertsCSV := make([][]string, 0, len(adverts)+1)
	advertsCSV = append(advertsCSV, GetJSONTags(adverts[0]))
	for _, advert := range adverts {
		advertsCSV = append(advertsCSV, StructToStringSlice(advert))
	}

	// writing to csv
	err = WriteExcelCSV("output.csv", advertsCSV)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// reading from csv

	advertsFromCSV, err := ReadExcelCSV("output.csv")
	if err != nil {
		fmt.Println("Error:", err)
	}

	//
	var advertsDownloaded []AvitoAdvert

	for _, v := range advertsFromCSV[1:] {

		priceVal, err := strconv.Atoi(v[3])

		var pricePtr *int
		if err == nil {
			pricePtr = &priceVal
		}

		hasM, _ := strconv.ParseBool(v[10])
		hasP, _ := strconv.ParseBool(v[11])

		advert := AvitoAdvert{
			ID:             v[0],     //string `json:"id"`
			Title:          v[1],     //string `json:"title"`
			URL:            v[2],     //string `json:"url"`
			Price:          pricePtr, //*int   `json:"price"`
			Currency:       v[4],     //string `json:"currency"`
			PriceText:      v[5],     //string `json:"priceText"`
			Description:    v[6],     //string `json:"description"`
			Location:       v[7],     //string `json:"location"`
			Rating:         v[8],     //string `json:"rating"`
			Reviews:        v[9],     //string `json:"reviews"`
			HasMessenger:   hasM,     //bool   `json:"hasMessenger"`
			HasPhoneButton: hasP,     //bool   `json:"hasPhoneButton"`

		}
		advertsDownloaded = append(advertsDownloaded, advert)
	}

	PrintTablesInTerminal(advertsFromCSV)

	//
	fmt.Println(advertsDownloaded)
}

func ReadExcelCSV(filePath string) ([][]string, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Can't read the file: %v", err)
	}

	bom := []byte{0xEF, 0xBB, 0xBF}
	cleanBytes, _ := bytes.CutPrefix(fileBytes, bom)

	r := csv.NewReader(bytes.NewReader(cleanBytes))
	r.Comma = ';'

	data, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV parsing error: %v", err)
	}
	return data, nil
}

func WriteExcelCSV(filePath string, data [][]string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM signature, to show correct Russian in Excel

	writer := csv.NewWriter(file)
	writer.Comma = ';' // for correct csv parsing by Excel
	for _, record := range data {
		err := writer.Write(record)
		if err != nil {
			return err
		}
	}

	writer.Flush()
	return nil
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

func PrintTablesInTerminal(stringBuffer [][]string) {
	if len(stringBuffer) < 2 {
		return
	}

	headers := stringBuffer[0]
	rowsData := stringBuffer[1:]

	for _, row := range rowsData {
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)

		id := "N/A"
		if len(row) > 0 {
			id = row[0]
		}

		t.AppendHeader(table.Row{"Property", fmt.Sprintf("ID: %s", id)})

		for colIdx := 1; colIdx < len(headers); colIdx++ {
			val := ""
			if colIdx < len(row) {
				val = row[colIdx]
			}
			t.AppendRow(table.Row{headers[colIdx], val})
		}

		// Настройки отображения карточки
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 1, WidthMax: 15, WidthMaxEnforcer: text.WrapText},
			{Number: 2, WidthMax: 60, WidthMaxEnforcer: text.WrapText},
		})

		t.SetStyle(table.StyleLight)
		t.Render()
		fmt.Println()
	}
}
