package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/gocolly/colly"
)

type ScrapeRequest struct {
	Url   string                `json:"url"`
	Items map[string]ScrapeItem `json:"items"`
}

type ScrapeItem struct {
	Selector string `json:"selector"`
	// TODO: udpate to map to interface, which is either a string
	// TODO: ... or a map of string to iface again with nested objs...
	Fields map[string]string `json:"fields"`
}

func main() {
	serveAddr := flag.String("serve", "127.0.0.1:6100", "Address:Port to serve http.")
	flag.Parse()
	http.HandleFunc("/scrape", scrape)
	log.Printf("Serving http on: %s", *serveAddr)
	http.ListenAndServe(*serveAddr, nil)
}

func scrape(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed.", http.StatusMethodNotAllowed)
		return
	}
	var scrapeReq ScrapeRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&scrapeReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: remove once done initial testing:
	// j, _ := json.MarshalIndent(scrapeReq, "", "    ")
	// fmt.Fprintf(w, "TODO: do it: %v, json: %s", scrapeReq, string(j))

	if err := validate(&scrapeReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := colly.NewCollector()
	results := make(map[string][]interface{})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL.String())
	})

	for iName, iVal := range scrapeReq.Items {
		c.OnHTML(iVal.Selector, func(e *colly.HTMLElement) {
			outItem := make(map[string][]interface{})
			for fName, _ := range iVal.Fields {
				// TODO: eventually support that fVal may itself be nested...
				// TODO: starting with e, select/extract value for each fiedl (fName)
				addValue(outItem, fName, "todo-value-for"+fName)
			}

			// TODO: get all field key-values
			// TODO: update to array of results not just last result...
			// TODO: support first/last/multi values?
			link := e.Attr("href")
			// Print link
			fmt.Printf("Link found: %q -> %s\n", e.Text, link)
			// TODO: make this an array of values:
			addValue(results, iName, outItem)
		})

	}

	var wg sync.WaitGroup
	wg.Add(1)
	c.OnScraped(func(r *colly.Response) {
		fmt.Println("Finished", r.Request.URL)
		wg.Done()
	})

	c.Visit(scrapeReq.Url)
	wg.Wait()

	if j, err := json.MarshalIndent(results, "", "    "); err == nil {
		io.WriteString(w, string(j))
	} else {
		log.Printf("ERROR: failed to marshal json, err: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func addValue(outMap map[string][]interface{}, key string, value interface{}) {
	if prev, found := outMap[key]; found {
		outMap[key] = append(prev, value)
	} else {
		outMap[key] = []interface{}{value}
	}
}

func validate(s *ScrapeRequest) error {
	if s == nil {
		return errors.New("request was nil")
	}
	if _, uErr := url.Parse(s.Url); uErr != nil {
		return uErr
	}
	if len(s.Items) == 0 {
		return errors.New("request.items was empty")
	}
	for itemK, itemV := range s.Items {
		if len(itemV.Selector) == 0 {
			return fmt.Errorf("request.items[%q].selector was empty", itemK)
		}
		if len(itemV.Fields) == 0 {
			return fmt.Errorf("request.items[%q].fields was empty", itemK)
		}
		for fieldK, fieldV := range itemV.Fields {
			if len(fieldK) == 0 || len(fieldV) == 0 {
				return fmt.Errorf("Empty request.items[%q].field key: %q, value: %q",
					itemK, fieldK, fieldV)
			}
		}
	}
	return nil
}
