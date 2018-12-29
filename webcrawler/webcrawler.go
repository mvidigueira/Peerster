package webcrawler

import (
	"crypto/sha1"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bbalet/stopwords"
)

/*func main() {

	crawler := New()
	crawler.Start()

	crawler.InChan <- &CrawlerPacket{
		HyperlinkPackage: &HyperlinkPackage{
			Links: []string{"/wiki/Sweden"},
		},
	}

	for {
		select {
		case packet := <-crawler.OutChan:
			switch {
			case packet.HyperlinkPackage != nil:
				fmt.Printf("Sending %d links to other nodes.\n", len(packet.HyperlinkPackage.Links))
			case packet.IndexPackage != nil:

			}
		}
	}
}*/

type Crawler struct {
	crawlQueue []string
	mux        *sync.Mutex
	domain     string
	InChan     chan *CrawlerPacket
	OutChan    chan *CrawlerPacket
	leader     bool
}

// PUBLIC API

func New(leader bool) *Crawler {
	return &Crawler{
		crawlQueue: []string{},
		mux:        &sync.Mutex{},
		domain:     "http://en.wikipedia.org",
		InChan:     make(chan *CrawlerPacket),
		OutChan:    make(chan *CrawlerPacket),
		leader:     leader,
	}
}

func (wc *Crawler) Start() {
	fmt.Println("Starting crawl...")
	go wc.crawl()
	go wc.listenToQueueUpdate()

	if wc.leader {
		wc.InChan <- &CrawlerPacket{
			HyperlinkPackage: &HyperlinkPackage{
				Links: []string{"/wiki/Sweden"},
			},
		}

	}
}

// PRIVATE METHODS

// Starts crawl loop
func (wc *Crawler) crawl() {

	go func() {
		for {
			select {
			case <-time.After(time.Second * 1):
				if len(wc.crawlQueue) == 0 {
					continue
				}
				// Get next page to crawl
				nextPage := wc.popQueue()

				// Crawl page
				page := wc.crawlUrl(nextPage)

				fmt.Printf("Crawled %s, found %d hyperlinks and %d keywords.\n", nextPage, len(page.Hyperlinks), len(page.Words))

				//urlsBelongingToMyDomain, urlsBelongingToOtherDomains := wc.divideURLsAfterDomain(page.Hyperlinks)
				// Update local crawl queue
				//wc.updateQueue(urlsBelongingToMyDomain)

				// Send urls belonging to other domains
				wc.OutChan <- &CrawlerPacket{
					HyperlinkPackage: &HyperlinkPackage{
						Links: page.Hyperlinks,
					},
				}

				// Send words to be indexed
				wc.OutChan <- &CrawlerPacket{
					IndexPackage: &IndexPackage{
						Keywords: page.Words,
						Url:      nextPage,
					},
				}
			}
		}
	}()
}

// Listen to crawler queue updates from other nodes
func (wc *Crawler) listenToQueueUpdate() {
	go func() {
		for {
			select {
			case packet := <-wc.InChan:
				switch {
				case packet.HyperlinkPackage != nil:
					wc.updateQueue(packet.HyperlinkPackage.Links)
				default:
					log.Fatal("Unknown packet.")
				}
			}
		}
	}()
}

// Crawls a url
func (wc *Crawler) crawlUrl(urlString string) *PageInfo {

	u, _ := url.ParseRequestURI(urlString)

	rawDoc := wc.getPage(urlString)

	doc := wc.cleanPage(*rawDoc)

	urls := wc.extractHyperLinks(doc, u.Host)

	words := wc.extractWords(doc)

	return &PageInfo{Hyperlinks: wc.removeDuplicates(urls), Words: wc.removeDuplicates(words)}
}

// Extracts words from a wikipedia document
func (wc *Crawler) extractWords(doc goquery.Document) []string {
	words := []string{}

	// Extract text, after inspecting the wikipedia website structure I noticed that all "interesting" text
	// was contained inside <p> tags hence I only extract text contained inside these tags.
	doc.Find("p").Each(func(i int, el *goquery.Selection) {

		// Extract text only, removes special characters and digits.
		processedString := wc.extractText(el.Text())

		// Remove stop words
		processedString = stopwords.CleanString(processedString, "en", true)

		// Tokenize
		tokens := strings.Split(processedString, " ")

		//Very slow -
		//lemmatizer, _ := golem.New("english")
		validWord := regexp.MustCompile(`^[a-zA-Z]+$`)
		for _, t := range tokens {
			word := t //lemmatizer.Lemma(t)
			// We make the assumption that most "valuable" words have a length greater than 2.
			if len(word) < 3 {
				continue
			}
			if !validWord.MatchString(word) {
				continue
			}
			words = append(words, word)
		}
	})
	return words
}

// Extracts local hyperlinks (links that does not point towards other domains than wikipedia)
func (wc *Crawler) extractHyperLinks(doc goquery.Document, host string) []string {
	urls := []string{}

	doc.Find("a").Each(func(i int, el *goquery.Selection) {
		href, exists := el.Attr("href")
		if exists {
			validURL := regexp.MustCompile(`^[a-zA-Z/_]+$`)
			if href[0] != '/' || !validURL.MatchString(href) {
				return
			}
			urls = append(urls, href)
		}
	})
	return urls
}

// Extracts only alphabetical characters from a text
func (wc *Crawler) extractText(text string) string {
	reg, err := regexp.Compile("[^a-zA-Z]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(text, " ")
}

// Removes unwanted parts of wikipedia HTML pages
func (wc *Crawler) cleanPage(doc goquery.Document) goquery.Document {
	doc.Find("script").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("head").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("style").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("img").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("span").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[id=mw-navigation]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[id=toc]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[id=Further_reading]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[id=External_links]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[id=catlinks]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.RemoveClass("reflist")

	return doc
}

// Downloads a page using HTTP
func (wc *Crawler) getPage(url string) *goquery.Document {
	res, err := http.Get(wc.domain + url)
	fmt.Println("http transport error is:", err)
	doc, err := goquery.NewDocumentFromReader(res.Body)
	fmt.Println("goquery error is:", err)
	pageTitle := doc.Find("title").Contents().Text()
	fmt.Printf("Page Title: '%s'\n", pageTitle)
	return doc
}

// Returns true if the url belongs to this nodes crawl domain
func (wc *Crawler) isMyDomain(url string) bool {
	return true
}

// Removes duplicates from the list given as input
func (wc *Crawler) removeDuplicates(strs []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range strs {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// Creates a 20byte hash
func (wc *Crawler) hash(id string) [20]byte {
	return sha1.Sum([]byte(id))
}

// Removes first element from crawler queue
func (wc *Crawler) popQueue() string {
	wc.mux.Lock()
	defer wc.mux.Unlock()
	head := wc.crawlQueue[0]
	wc.crawlQueue = wc.crawlQueue[1:]
	return head
}

// Updates crawler queue
func (wc *Crawler) updateQueue(hyperlinks []string) {
	wc.mux.Lock()
	defer wc.mux.Unlock()
	wc.crawlQueue = append(wc.crawlQueue, hyperlinks...)
}

func (wc *Crawler) DivideURLsAfterDomain(urls []string) ([]string, []string) {
	myDomain := []string{}
	otherDomains := []string{}
	for _, url := range urls {
		if wc.isMyDomain(url) {
			myDomain = append(myDomain, url)
		} else {
			otherDomains = append(otherDomains, url)
		}
	}
	return myDomain, otherDomains
}
