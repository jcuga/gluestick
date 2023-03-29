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
	"reflect"
	"strings"
	"sync"

	"github.com/gocolly/colly"
)

type ScrapeRequest struct {
	Url   string                `json:"url"`
	Items map[string]ScrapeItem `json:"items"`
}

type ScrapeItem struct {
	Selector string `json:"selector"`
	// Fields can be a single name->valueSelector, or nested name->{n1->s1, n2->s2, etc }}
	// The field's valueSelectors can be a selector in which case the ChildText()
	// is called. Or "selector|attr" to specify which ChildAttrs() is used.
	// OR simple "|attr" to get Attr() directly on parent selected element.
	Fields map[string]interface{} `json:"fields"`
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

	if err := validate(&scrapeReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := colly.NewCollector()
	results := make(map[string]interface{})

	c.OnRequest(func(r *colly.Request) {
		log.Println("Scraping", r.URL.String())
	})

	for itemName, item := range scrapeReq.Items {
		c.OnHTML(item.Selector, func(e *colly.HTMLElement) {
			parsed := parseFields(item.Fields, e)
			accumValue(results, itemName, parsed)
		})

	}

	var wg sync.WaitGroup
	wg.Add(1)
	c.OnScraped(func(r *colly.Response) {
		log.Println("Finished", r.Request.URL)
		wg.Done()
	})
	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong:", err)
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

func parseFields(fields map[string]interface{}, e *colly.HTMLElement) map[string]interface{} {
	parsed := make(map[string]interface{})
	for fieldName, field := range fields {
		if fieldSelector, ok := field.(string); ok {
			sel, attr := getSelectorAndAttr(fieldSelector)
			if len(sel) == 0 {
				if len(attr) == 0 { // Use text
					accumValue(parsed, fieldName, e.Text)
				} else { // Use attr
					accumValue(parsed, fieldName, e.Attr(attr))
				}
			} else {
				if len(attr) == 0 {
					accumValue(parsed, fieldName, e.ChildText(sel))
				} else {
					for _, val := range e.ChildAttrs(sel, attr) {
						accumValue(parsed, fieldName, val)
					}
				}
			}
		} else if nestedFields, ok := field.(map[string]interface{}); ok {
			val := parseFields(nestedFields, e)
			accumValue(parsed, fieldName, val)
		} else {
			log.Printf("ERROR: expected string or map[string]interface{}, got: %s\n", reflect.TypeOf(field))
		}
	}
	return parsed
}

// Store single/multi values to map.  On first set, single value.
// On subsequent set's, upgrade value to a slice and append.
// This allows easy value accumulation without having to specify up front
// if we're wanting a single or multi value.
func accumValue(outMap map[string]interface{}, key string, value interface{}) {
	if prev, found := outMap[key]; found {
		if multi, ok := prev.([]interface{}); ok {
			// already a slice, append
			outMap[key] = append(multi, value)
		} else {
			// prev was single value, convert to slice and add new val
			outMap[key] = []interface{}{prev, value}
		}
	} else {
		outMap[key] = value
	}
}

func getSelectorAndAttr(input string) (string, string) {
	idx := strings.LastIndex(input, "|")
	if idx == -1 {
		// selector only--no "|attr" specified
		return input, ""
	}
	// TODO: handle trailing | ?
	// TODO: handle when both are blank? or calling code already has a meaning for this
	return input[:idx], input[idx+1:]
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
		// TODO: recursively validate all field leafs?
		// NOTE: can have an empty value (no selector|attribute) in which case
		// the parent's full text is used.
	}
	return nil
}
