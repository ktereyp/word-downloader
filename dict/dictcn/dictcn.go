package dictcn

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"word-downloader/dict"
)

type Word struct {
	W        string
	Audio    Audio
	BasicDef []BasicDefinition
	Defs     []Dict
}

func (w Word) Word() string {
	return w.W
}

func (w Word) Json() string {
	buf, _ := json.Marshal(w)
	return string(buf)
}

func (w Word) Type() dict.Dictionary {
	return dict.Dictcn
}

func (w Word) Mp3() []string {
	return []string{
		w.Audio.Us.FemaleMp3,
	}
}

func (w Word) Pronunciation() string {
	return fmt.Sprintf("/%v/", w.Audio.Us.Pronunciation)
}

func (w Word) DefinitionHtml(showHeadWord bool) string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="word-content">`)
	sb.WriteString(fmt.Sprintf(`<div class="dict-name">%v</div>`, w.Type().Name()))

	if showHeadWord {
		sb.WriteString(`<div class="this-word">`)
		sb.WriteString(w.W)
		sb.WriteString(`</div>`)
	}

	sb.WriteString(`<div class="basic-def-list">`)
	for _, b := range w.BasicDef {
		sb.WriteString(b.Html())
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`</div>`)
	return sb.String()
}

var _ dict.Word = Word{}

type BasicDefinition struct {
	ParOfSpeech string
	Def         string
}

func (b BasicDefinition) Html() string {
	return fmt.Sprintf(`<div class="basic-def>"><span class="pos">%v</span>%v</div>`, b.ParOfSpeech, b.Def)
}

type Dict struct {
	DictName   string
	DefEntries []DefinitionEntry
}

type DefinitionEntry struct {
	PartOfSpeech       string
	SubDefinitionEntry []SubDefinition
}

func (d DefinitionEntry) Html() string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="sub-def-list">`)
	sb.WriteString(`<table class="table-align">`)

	for _, subDef := range d.SubDefinitionEntry {
		sb.WriteString(`<tr class="table-align">`)
		sb.WriteString(`<td class="table-align">`)
		sb.WriteString(subDef.Html())
		sb.WriteString(`</td>`)
		sb.WriteString(`</tr>`)
	}
	sb.WriteString(`</table>`)
	sb.WriteString(`</div>`)
	return sb.String()
}

type SubDefinition struct {
	Def      string
	Examples []Example
}

func (s SubDefinition) Html() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf(`<div class="sub-def-content">`))
	sb.WriteString(`<div class="sub-def">`)
	sb.WriteString(s.Def)
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="use-examples">`)
	for _, exp := range s.Examples {
		sb.WriteString(exp.Html())
	}
	sb.WriteString("</div>")
	sb.WriteString("</div>")
	return sb.String()
}

type Audio struct {
	Us struct {
		Pronunciation string
		MaleMp3       string
		FemaleMp3     string
	}
	Uk struct {
		Pronunciation string
		MaleMp3       string
		FemaleMp3     string
	}
}

type Example struct {
	Text string
}

func (e Example) Html() string {
	return fmt.Sprintf(`<div class=use-example>// %v</div>`, e.Text)
}

type dictcnDict struct {
	httpClient *http.Client
}

func (dictcn *dictcnDict) Parse(wordJson []byte) (dict.Word, error) {
	var word Word
	err := json.Unmarshal(wordJson, &word)
	return word, err
}

func NewDict() *dictcnDict {
	return &dictcnDict{
		httpClient: &http.Client{
			Transport: &http.Transport{
				Proxy:               nil,
				DialContext:         (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 256,
				IdleConnTimeout:     time.Minute * 10,
			},
		},
	}
}

func (dictcn *dictcnDict) Type() dict.Dictionary {
	return dict.Dictcn
}

func (dictcn *dictcnDict) Lookup(word string) (dict.Word, error) {
	col := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36"),
	)
	col.SetClient(dictcn.httpClient)

	out := Word{}

	col.OnHTML(".keyword", func(element *colly.HTMLElement) {
		out.W = element.Text
	})
	col.OnHTML(".phonetic", func(element *colly.HTMLElement) {
		element.DOM.Find("span").Each(func(i int, selection *goquery.Selection) {
			if i == 0 {
				out.Audio.Uk.Pronunciation = selection.Find("bdo").Text()
				out.Audio.Uk.FemaleMp3, _ = selection.Find("i:nth-child(2)").Attr("naudio")
				out.Audio.Uk.MaleMp3, _ = selection.Find("i:nth-child(3)").Attr("naudio")
			} else {
				out.Audio.Us.Pronunciation = selection.Find("bdo").Text()
				out.Audio.Us.FemaleMp3, _ = selection.Find("i:nth-child(2)").Attr("naudio")
				out.Audio.Us.MaleMp3, _ = selection.Find("i:nth-child(3)").Attr("naudio")
			}
		})
		if out.Audio.Uk.FemaleMp3 != "" {
			out.Audio.Uk.FemaleMp3 = "http://audio.dict.cn/" + out.Audio.Uk.FemaleMp3
		}
		if out.Audio.Uk.MaleMp3 != "" {
			out.Audio.Uk.MaleMp3 = "http://audio.dict.cn/" + out.Audio.Uk.MaleMp3
		}
		if out.Audio.Us.FemaleMp3 != "" {
			out.Audio.Us.FemaleMp3 = "http://audio.dict.cn/" + out.Audio.Us.FemaleMp3
		}
		if out.Audio.Us.MaleMp3 != "" {
			out.Audio.Us.MaleMp3 = "http://audio.dict.cn/" + out.Audio.Us.MaleMp3
		}
	})

	extractDict := func(element *colly.HTMLElement, dictName string) {
		var entries []DefinitionEntry
		element.DOM.Find("span").Each(func(i int, selection *goquery.Selection) {
			selection.Find("bdo").Remove()
			entries = append(entries, DefinitionEntry{
				PartOfSpeech:       strings.TrimSpace(selection.Text()),
				SubDefinitionEntry: nil,
			})
		})
		element.DOM.Find("ol").Each(func(i int, selection *goquery.Selection) {
			if i < len(entries) {
				selection.Find("li").Each(func(_ int, selection *goquery.Selection) {
					// example
					examples := strings.Split(selection.Find("p").Text(), "\n")
					var exampleSlice []Example
					for _, e := range examples {
						e = strings.TrimSpace(e)
						if e != "" {
							exampleSlice = append(exampleSlice, Example{e})
						}
					}
					selection.Find("p").Remove()

					entries[i].SubDefinitionEntry = append(entries[i].SubDefinitionEntry, SubDefinition{
						Def:      strings.TrimSpace(selection.Text()),
						Examples: exampleSlice,
					})
				})
			}
		})
		out.Defs = append(out.Defs, Dict{
			DictName:   dictName,
			DefEntries: entries,
		})
	}

	col.OnHTML(".layout.detail", func(element *colly.HTMLElement) {
		extractDict(element, "详尽释义")
	})
	col.OnHTML(".layout.dual", func(element *colly.HTMLElement) {
		extractDict(element, "双解释义")
	})
	col.OnHTML(".layout.en", func(element *colly.HTMLElement) {
		extractDict(element, "英英释义")
	})

	col.OnHTML(".dict-basic-ul", func(element *colly.HTMLElement) {
		element.DOM.Find("li").Each(func(i int, selection *goquery.Selection) {
			basic := BasicDefinition{
				ParOfSpeech: selection.Find("span").Text(),
				Def:         selection.Find("strong").Text(),
			}
			if basic.Def != "" {
				out.BasicDef = append(out.BasicDef, basic)
			}
		})
	})

	col.OnRequest(func(r *colly.Request) {
		r.Headers.Add("accept", "*/*")
	})

	urlEncodedWord := url.QueryEscape(word)
	searchUrl := fmt.Sprintf(
		"http://dict.cn/%v",
		urlEncodedWord,
	)
	err := col.Visit(searchUrl)
	if err != nil {
		return Word{}, err
	}

	if out.W == "" {
		return Word{}, dict.ErrNotFound
	}

	return out, nil
}

func partOfSpeech(pos string) string {
	if pos == "transitive verb" {
		return "vt."
	}
	if pos == "intransitive verb" {
		return "v."
	}
	return pos
}
