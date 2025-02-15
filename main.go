package main

import (
	"errors"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"log"
	"net/url"
	// "sort"
	"strings"
	"time"

	"github.com/gophercises/quiet_hn/hn"
)

type result struct {
	idx int
	item item
	err error
}

var (
	cache           []item
	cacheExpiration time.Time
)

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		stories, err := getCachedStories(numStories)
		// stories,  err := getTopStories(numStories)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := templateData{
			Stories: stories,
			Time: time.Now().Sub(start),
		}

		fmt.Println("TIME IS: ", data.Time)

		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

func getCachedStories(numStories int) ([]item, error) {
	if time.Now().Sub(cacheExpiration) < 0 {
		return cache, nil
	}
	stories, err := getTopStories(numStories)
	if err != nil {
		return nil, err
	}
	cache = stories
	cacheExpiration = time.Now().Add(60 * time.Second)
	return cache, nil
}

// CONCURRENCY WITH MAINTAINED ORDERING
// func getTopStories(numStories int) ([]item, error) {
// 	var client hn.Client
// 	ids, err := client.TopItems()
// 	if err != nil {
// 		return nil, errors.New("Failed to load top stories")
// 	}
//
// 	var stories []item
// 	resultCh := make(chan result)
//
// 	for i := 0; i < numStories; i++ {
// 		go func(idx, id int) {
// 			hnItem, err := client.GetItem(id)
// 			if err != nil {
// 				resultCh <- result{idx: idx, err: err}
// 			}
// 			resultCh <- result{idx: idx, item: parseHNItem(hnItem)}
// 		}(i, ids[i])
// 	}
//
// 	var results []result
//
// 	for i := 0; i < numStories; i++ {
// 		results = append(results, <- resultCh)
// 	}
//
// 	sort.Slice(results, func(i, j int) bool {
// 		return results[i].idx < results[j].idx
// 	})
//
// 	for _, res := range results {
// 		if res.err != nil {
// 			continue
// 		}
//
// 		if isStoryLink(res.item) {
// 			stories = append(stories, res.item)
// 		}
// 	}
//
// 	return stories, nil
// }

// FIRST ATTEMPT TO REFACTOR
func getTopStories(numStories int) ([]item, error) {
	var client hn.Client
	ids, err := client.TopItems()
	if err != nil {
		return nil, errors.New("Failed to load top stories")
	}

	var stories []item

	for _, id := range ids {
		resultCh := make(chan result)
		go func(id int) {
			hnItem, err := client.GetItem(id)
			if err != nil {
				resultCh <- result{err: err}
			}
			resultCh <- result{item: parseHNItem(hnItem)}
		}(id)

		res := <- resultCh
		if res.err != nil {
			continue
		}

		if isStoryLink(res.item) {
			stories = append(stories, res.item)
			if len(stories) >= numStories {
				break
			}
		}
	}

	return stories, nil
}

func isStoryLink(item item) bool {
	return item.Type == "story" && item.URL != ""
}

func parseHNItem(hnItem hn.Item) item {
	ret := item{Item: hnItem}
	url, err := url.Parse(ret.URL)
	if err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return ret
}

// item is the same as the hn.Item, but adds the Host field
type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))
	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
