package webcrawler

import (
	"fmt"
	"hash"
	"hash/fnv"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/bbalet/stopwords"
	"github.com/mvidigueira/Peerster/bloomfilter"
	"github.com/mvidigueira/Peerster/dht"
)

type Crawler struct {
	crawlQueue  []string
	mux         *sync.Mutex
	domain      string
	InChan      chan *CrawlerPacket
	OutChan     chan *CrawlerPacket
	leader      bool
	bloomFilter *bloomfilter.BloomFilter
	hasher      hash.Hash64
}

// PUBLIC API

func New(leader bool) *Crawler {
	return &Crawler{
		crawlQueue:  []string{},
		mux:         &sync.Mutex{},
		domain:      "http://en.wikipedia.org",
		InChan:      make(chan *CrawlerPacket),
		OutChan:     make(chan *CrawlerPacket),
		leader:      leader,
		bloomFilter: bloomfilter.New(3, 10e6),
		hasher:      fnv.New64(),
	}
}

func (wc *Crawler) Start() {
	fmt.Println("Starting crawl...")
	go wc.crawl()
	go wc.listenToQueueUpdate()

	if wc.leader {
		wc.InChan <- &CrawlerPacket{
			HyperlinkPackage: &HyperlinkPackage{
				Links: []string{"/wiki/Swedish_Empire"},
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

				page := wc.crawlUrl(nextPage)

				fmt.Printf("Crawled %s, found %d hyperlinks and %d keywords.\n", nextPage, len(page.Hyperlinks), len(page.KeywordFrequencies))

				// Lookup the page hash in the DHT in order to prevent parsing of a page which has already been crawled by another node
				// This could be the case since the same page could be pointed to by several URLs.
				resChan := make(chan bool)
				wc.OutChan <- &CrawlerPacket{
					PageHash: &PageHashPackage{
						Hash: page.Hash,
						Type: "lookup",
					},
					ResChan: resChan,
				}
				// wait for response
				found := <-resChan
				if found {
					fmt.Printf("Page has already been crawled, skipping.\n")
					continue
				}

				// Filter out urls that already has been crawler by this crawler
				filteredHyperLinks := make([]string, 0, len(page.Hyperlinks))
				for _, hyperlink := range page.Hyperlinks {
					if !wc.bloomFilter.IsSet([]byte(hyperlink)) {
						filteredHyperLinks = append(filteredHyperLinks, hyperlink)
					}
				}

				// Send the links found on the page to be distributed evenly between the availible crawlers
				wc.OutChan <- &CrawlerPacket{
					HyperlinkPackage: &HyperlinkPackage{
						Links: filteredHyperLinks,
					},
				}

				// Send the hash of the page content to be stored in the DHT
				wc.OutChan <- &CrawlerPacket{
					PageHash: &PageHashPackage{
						Hash: page.Hash,
						Type: "store",
					},
				}

				// Send words to be indexed
				wc.OutChan <- &CrawlerPacket{
					IndexPackage: &IndexPackage{
						KeywordFrequencies: page.KeywordFrequencies,
						Url:                nextPage,
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

	return &PageInfo{Hyperlinks: wc.removeDuplicates(urls), KeywordFrequencies: wc.keywordFrequency(words), Hash: dht.GenerateKeyHash(rawDoc.Text())}
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
			word = strings.ToLower(word)
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
	doc.Find("div[id=footer]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})
	doc.Find("div[class=nowraplinks]").Each(func(i int, el *goquery.Selection) {
		el.Remove()
	})

	doc.RemoveClass("reflist")
	doc.RemoveClass("nowraplinks")

	return doc
}

// Downloads a page using HTTP
func (wc *Crawler) getPage(url string) *goquery.Document {
	fmt.Println(wc.domain + url)
	res, err := http.Get(wc.domain + url)
	if err != nil {
		fmt.Println("http transport error is:", err)
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		fmt.Println("goquery error is:", err)
		return nil
	}
	pageTitle := doc.Find("title").Contents().Text()
	fmt.Printf("Page Title: '%s'\n", pageTitle)
	return doc
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

// Removes duplicates from the list given as input
func (wc *Crawler) keywordFrequency(keywords []string) map[string]int {
	frequencies := make(map[string]int)
	for _, keyword := range keywords {
		val, found := frequencies[keyword]
		if !found {
			frequencies[keyword] = 1
			continue
		}
		frequencies[keyword] = val + 1
	}
	return frequencies
}

// creates a 20 byte long hash
/*func (wc *Crawler) fastHash(id string) dht.TypeID {
	wc.hasher.Reset()
	wc.hasher.Write([]byte(id))
	hash := wc.hasher.Sum64()
	var b [dht.IDByteSize]byte
	binary.LittleEndian.PutUint64(b[:], hash)
	return b
}*/

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
	for _, hyperlink := range hyperlinks {
		if wc.bloomFilter.IsSet([]byte(hyperlink)) {
			continue
		}
		wc.crawlQueue = append(wc.crawlQueue, hyperlink)
	}
}
