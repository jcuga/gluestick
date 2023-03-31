package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"reflect"
	"strings"

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

type ScrapeResult map[string]interface{}

func main() {
	inFilename := flag.String("f", "", "Input json filename.")
	inString := flag.String("in", "", "Input json directly.")
	doVerbose := flag.Bool("v", false, "Verbose output.")
	flag.Parse()

	var inputJson []byte
	if len(*inString) > 0 {
		inputJson = []byte(*inString)
	} else if len(*inFilename) > 0 {
		inBytes, err := ioutil.ReadFile(*inFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open input file: %q, error: %s\n", *inFilename, err)
			os.Exit(1)
		}
		inputJson = inBytes
	} else {
		inBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read from stdin, error: %s\n", err)
			os.Exit(1)
		}
		inputJson = inBytes
	}

	var scrapeReq ScrapeRequest
	if err := json.Unmarshal(inputJson, &scrapeReq); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse input as json request, error: %s\n", err)
		os.Exit(1)
	}
	if err := validate(&scrapeReq); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid scrape request: %s\n", err)
		os.Exit(1)
	}

	results, err := scrape(scrapeReq, *doVerbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while scraping: %s\n", err)
		os.Exit(1)
	}

	if j, err := json.MarshalIndent(results, "", "    "); err == nil {
		fmt.Fprintln(os.Stdout, string(j))
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "failed to marshal results as json, error: %v\n", err)
		os.Exit(1)
	}
}

func scrape(req ScrapeRequest, verbose bool) (ScrapeResult, error) {
	c := colly.NewCollector()
	results := make(map[string]interface{})

	c.OnRequest(func(r *colly.Request) {
		if verbose {
			log.Println("Scraping", r.URL.String())
		}
	})

	for itemName, item := range req.Items {
		// NOTE: have to capture itemName, item else will only get last in loop:
		func(name string, i ScrapeItem) {
			c.OnHTML(i.Selector, func(e *colly.HTMLElement) {
				parsed := parseFields(i.Fields, e)
				accumValue(results, name, parsed)
			})
		}(itemName, item)
	}

	scrapeError := make(chan error, 1)
	c.OnScraped(func(r *colly.Response) {
		if verbose {
			log.Println("Finished", r.Request.URL)
		}
		close(scrapeError)
	})
	c.OnError(func(_ *colly.Response, err error) {
		if verbose {
			log.Println("Something went wrong:", err)
		}
		scrapeError <- err
	})
	c.Visit(req.Url)
	scrapeErr := <-scrapeError
	return results, scrapeErr
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
					e.ForEach(sel, func(i int, child *colly.HTMLElement) {
						accumValue(parsed, fieldName, child.Text)
					})
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
		return strings.TrimSpace(input), ""
	}
	return strings.TrimSpace(input[:idx]), strings.TrimSpace(input[idx+1:])
}

func validate(req *ScrapeRequest) error {
	if req == nil {
		return errors.New("request was nil")
	}
	if _, uErr := url.Parse(req.Url); uErr != nil {
		return uErr
	}
	if len(req.Items) == 0 {
		return errors.New("request.items was empty")
	}
	for itemK, itemV := range req.Items {
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
