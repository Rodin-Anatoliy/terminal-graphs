//package terminal_graphs
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/gosuri/uilive"
	"github.com/guptarohit/asciigraph"
)

// необходимые параметры

// Trade - структура для торговых сделок
type Trade struct {
	ID        int64  `json:"trade_id"`
	Type      string `json:"type"`
	Pair      string `json:"pair"`
	Price     string `json:"price"`
	Amount    string `json:"amount"`
	Timestamp int64  `json:"timestamp"`
}

type Data struct {
	prices   []float64
	pairsKey string
}

type Api struct {
	url string
}

func (api *Api) getPrices(pairs string) ([]float64, string, error) {
	req, err := http.NewRequest("GET", api.url+"/trades?pair="+pairs, nil)
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var tradesResponse map[string][]Trade
	err = json.NewDecoder(resp.Body).Decode(&tradesResponse)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %v", err)
	}

	trades, ok := tradesResponse[pairs]
	if !ok {
		return nil, "", fmt.Errorf("API returned empty response for pair: %s", pairs)
	}

	prices := make([]float64, len(trades))
	for i, trade := range trades {
		price, err := strconv.ParseFloat(trade.Price, 64)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert trade.Price to float64: %v", err)
		}
		prices[i] = price
	}

	return prices, pairs, nil
}

func displayMenu(writer *uilive.Writer) {
	writer.Flush()
	fmt.Fprintln(writer, "1. BTC_USD")
	fmt.Fprintln(writer.Newline(), "2. LTC_USD")
	fmt.Fprintln(writer.Newline(), "3. ETH_USD")
	fmt.Fprintln(writer.Newline(), "")
	fmt.Fprintln(writer.Newline(), "Press 1-3 to change symbol, press q to exit")
}

func inputHandler(keysEvents <-chan keyboard.KeyEvent, pairsKeyCh chan string, displayModeCh chan int) {
	for range time.Tick(time.Millisecond * 100) {
		event := <-keysEvents
		if event.Err != nil {
			panic(event.Err)
		}
		switch event.Rune {
		case 'q':
			os.Exit(0)
		case '1':
			pairsKeyCh <- "BTC_USD"
			displayModeCh <- 1
		case '2':
			pairsKeyCh <- "LTC_USD"
			displayModeCh <- 1
		case '3':
			pairsKeyCh <- "ETH_USD"
			displayModeCh <- 1
		}

		if event.Key == keyboard.KeyBackspace || event.Key == keyboard.KeyBackspace2 {
			displayModeCh <- 0
		}
	}
}

func displayGraph(data Data, writer *uilive.Writer) {
	prices := data.prices
	pairsKey := data.pairsKey
	if len(prices) == 0 {
		return
	}

	writer.Flush()
	t := time.Now()

	graph := asciigraph.Plot(prices, asciigraph.Width(100), asciigraph.Height(10))
	fmt.Fprintf(writer, "%s: %.2f\n", pairsKey, prices[len(prices)-1])
	fmt.Fprintln(writer.Newline(), graph)
	fmt.Fprintln(writer.Newline(), "Текущая дата:", t.Format("2006-01-02"))
	fmt.Fprintln(writer.Newline(), "Текущее время:", t.Format("15:04:05"))
}

func displayWorker(dataCh chan Data, displayModeCh chan int, writer *uilive.Writer) {
	currentMode := -1
	for range time.Tick(time.Millisecond * 100) {
		select {
		case mode := <-displayModeCh:
			if mode != currentMode {
				currentMode = mode
				switch mode {
				case 0:
					displayMenu(writer)
				case 1:
					select {
					case data := <-dataCh:
						displayGraph(data, writer)
					default:
					}
				}
			}
		default:
			if currentMode == 1 {
				select {
				case data := <-dataCh:
					displayGraph(data, writer)
				default:
				}
			}
		}
	}
}

func dataWorker(dataCh chan Data, pairsKeyCh chan string) {
	api := &Api{url: "https://api.exmo.com/v1.1"}
	var lastPairsKey string

	for range time.Tick(time.Second) {
		select {
		case pairsKey := <-pairsKeyCh:
			lastPairsKey = pairsKey
		default:
		}

		if len(lastPairsKey) == 0 {
			continue
		}
		prices, key, err := api.getPrices(lastPairsKey)
		if err != nil {
			log.Println("Error fetching data:", err)
		}

		dataCh <- Data{prices: prices, pairsKey: key}
	}
}

func main() {
	keysEvents, err := keyboard.GetKeys(1)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = keyboard.Close()
	}()

	writer := uilive.New()
	writer.Start()
	defer writer.Stop()

	dataCh := make(chan Data, 1)
	pairsKeyCh := make(chan string, 1)
	displayModeCh := make(chan int, 1)

	go dataWorker(dataCh, pairsKeyCh)
	go displayWorker(dataCh, displayModeCh, writer)
	displayModeCh <- 0
	inputHandler(keysEvents, pairsKeyCh, displayModeCh)
}
