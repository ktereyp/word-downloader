package bingdict

import (
	"word-downloader/dict"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type bingDict struct {
	transport *http.Transport
}

func NewBingDict() *bingDict {
	return &bingDict{transport: &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     time.Minute * 10,
	},
	}
}

func (bing *bingDict) Lookup(word string) (dict.Word, error) {
	col := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36"),
	)
	col.SetClient(&http.Client{
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConns:        256,
			MaxIdleConnsPerHost: 256,
			IdleConnTimeout:     time.Minute * 10,
		},
	})

	out := dict.Word{}

	// get head word
	col.OnHTML("#headword > h1:nth-child(1) > strong:nth-child(1)", func(element *colly.HTMLElement) {
		out.W = element.Text
		log.Printf("word: %v", out.W)
	})

	// get pronunciation
	col.OnHTML(".hd_prUS", func(element *colly.HTMLElement) {
		out.Audio.PronunciationUS = element.Text
	})
	col.OnHTML("div.hd_tf:nth-child(2) > a:nth-child(1)", func(element *colly.HTMLElement) {
		onclickText := element.Attr("onclick")
		startPos := strings.Index(onclickText, "https")
		if startPos > 0 {
			endPos := strings.Index(onclickText, ".mp3")
			if endPos > startPos {
				out.Audio.USAudio = onclickText[startPos : endPos+4]
			}
		}
	})

	col.OnHTML(".hd_pr", func(element *colly.HTMLElement) {
		out.Audio.PronunciationUK = element.Text
	})
	col.OnHTML("div.hd_tf:nth-child(4) > a:nth-child(1)", func(element *colly.HTMLElement) {
		onclickText := element.Attr("onclick")
		startPos := strings.Index(onclickText, "https")
		if startPos > 0 {
			endPos := strings.Index(onclickText, ".mp3")
			if endPos > startPos {
				out.Audio.UKAudio = onclickText[startPos : endPos+4]
			}
		}
	})

	// simple definition
	var simpleDef dict.Definition
	var plural string
	col.OnHTML(".qdef > ul:nth-child(2)", func(element *colly.HTMLElement) {
		html, _ := element.DOM.Html()
		simpleDef.Kind = "simple-def"
		simpleDef.Raw = html
		simpleDef.Def = element.Text
	})
	col.OnHTML(".hd_div1", func(element *colly.HTMLElement) {
		text := element.DOM.Text()
		element.DOM.SetHtml(text)
		plural = text
	})

	// get definition
	col.OnHTML("#authid", func(element *colly.HTMLElement) {
		element.DOM.Find(".hw_area2").Remove()
		element.DOM.Find(".switch").Remove()
		element.DOM.Find(".se_d.b_primtxt").Each(func(i int, selection *goquery.Selection) {
			text := selection.Text()
			selection.SetHtml(text)
		})
		html, _ := element.DOM.Html()

		out.Defs = append(out.Defs, dict.Definition{
			Kind: "auth",
			Def:  element.Text,
			Raw:  html,
		})
	})
	col.OnHTML("#homoid", func(element *colly.HTMLElement) {
		element.DOM.Find(".df_cr_w").Each(func(i int, selection *goquery.Selection) {
			text := selection.Text()
			selection.SetHtml(text)
		})

		html, _ := element.DOM.Html()

		out.Defs = append(out.Defs, dict.Definition{
			Kind: "homo",
			Def:  element.DOM.Text(),
			Raw:  html,
		})
	})
	col.OnHTML("#crossid", func(element *colly.HTMLElement) {
		html, _ := element.DOM.Html()
		out.Defs = append(out.Defs, dict.Definition{
			Kind: "cross",
			Def:  element.Text,
			Raw:  html,
		})
	})

	// examples
	col.OnHTML("#sentenceSeg", func(element *colly.HTMLElement) {
		element.DOM.Find(".sen_ime.b_regtxt").Remove()
		element.DOM.Find(".sen_li.b_regtxt").Remove()
		element.DOM.Find(".mm_div").Remove()
		element.DOM.Find(".b_pag.b_cards").Remove()
		element.DOM.Find(".sen_en.b_regtxt").Each(func(i int, selection *goquery.Selection) {
			text := selection.Text()
			selection.SetHtml(text)
		})
		element.DOM.Find(".sen_cn.b_regtxt").Each(func(i int, selection *goquery.Selection) {
			text := selection.Text()
			selection.SetHtml(text)
		})

		html, _ := element.DOM.Html()
		out.Examples = append(out.Examples, dict.Example{
			Text: element.DOM.Text(),
			Raw:  html,
		})
	})

	col.OnRequest(func(r *colly.Request) {
		r.Headers.Add("accept", "*/*")
	})

	urlEncodedWord := url.QueryEscape(word)
	searchUrl := fmt.Sprintf(
		"https://cn.bing.com/dict/search?q=%v&qs=n&form=Z9LH5&sp=-1&pq=kes&sc=4-3&sk=",
		urlEncodedWord,
	)
	err := col.Visit(searchUrl)
	if err != nil {
		return dict.Word{}, err
	}

	if simpleDef.Raw != "" {
		simpleDef.Raw = fmt.Sprintf(`<div class="simple-def">%v</div> <div class="word-plural">%v</div>`,
			simpleDef.Raw, plural)
		out.Defs = append([]dict.Definition{simpleDef}, out.Defs...)
	}

	if out.W == "" {
		return dict.Word{}, dict.ErrNotFound
	}

	return out, nil
}
