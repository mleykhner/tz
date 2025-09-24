package main

import (
	"encoding/xml"
	"fmt"
	"golang.org/x/net/html/charset"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ValCurs struct {
	Date    string   `xml:"Date,attr"`
	Valutes []Valute `xml:"Valute"`
}

type Valute struct {
	CharCode  string `xml:"CharCode"`
	Name      string `xml:"Name"`
	VunitRate string `xml:"VunitRate"`
}

type Record struct {
	Date time.Time
	Code string
	Name string
	Rate float64
}

func fetchRates(client *http.Client, date time.Time) ([]Record, error) {
	url := fmt.Sprintf("https://www.cbr.ru/scripts/XML_daily_eng.asp?date_req=%02d/%02d/%d",
		date.Day(), int(date.Month()), date.Year())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/xml, text/xml, */*")
	req.Header.Set("Accept-Language", "ru-RU,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var valCurs ValCurs
	decoder := xml.NewDecoder(resp.Body)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(&valCurs); err != nil {
		return nil, fmt.Errorf("xml decode: %w", err)
	}

	var records []Record
	for _, v := range valCurs.Valutes {
		value := strings.ReplaceAll(v.VunitRate, ",", ".")
		rate, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		records = append(records, Record{
			Date: date,
			Code: v.CharCode,
			Name: v.Name,
			Rate: rate,
		})
	}
	return records, nil
}

func main() {
	end := time.Now()
	start := end.AddDate(0, 0, -90)

	var all []Record
	var mu sync.Mutex
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	days := int(end.Sub(start).Hours()/24) + 1
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		wg.Add(1)
		go func(date time.Time) {
			defer wg.Done()
			records, err := fetchRates(client, date)
			if err != nil {
				fmt.Printf("Нет информации за %s: %v\n", date.Format("2006-01-02"), err)
				return
			}
			mu.Lock()
			all = append(all, records...)
			mu.Unlock()
		}(d)
	}

	wg.Wait()

	if len(all) == 0 {
		fmt.Println("Нет данных за период")
		return
	}

	maxRec := all[0]
	minRec := all[0]
	var sum float64
	for _, r := range all {
		if r.Rate > maxRec.Rate {
			maxRec = r
		}
		if r.Rate < minRec.Rate {
			minRec = r
		}
		sum += r.Rate
	}
	avg := sum / float64(len(all))

	fmt.Println("Максимальный курс:")
	fmt.Printf("%s (%s) = %.6f, дата: %s\n", maxRec.Name, maxRec.Code, maxRec.Rate, maxRec.Date.Format("2006-01-02"))

	fmt.Println("Минимальный курс:")
	fmt.Printf("%s (%s) = %.6f, дата: %s\n", minRec.Name, minRec.Code, minRec.Rate, minRec.Date.Format("2006-01-02"))

	fmt.Printf("Среднее значение курса по всем валютам: %.6f\n", avg)
}
