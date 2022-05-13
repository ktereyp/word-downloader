package webster

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
	W     string
	Audio Audio
	Defs  []Definition
}

func (w Word) Word() string {
	return w.W
}

func (w Word) Json() string {
	buf, _ := json.Marshal(w)
	return string(buf)
}

func (w Word) Type() dict.Dictionary {
	return dict.Webster
}

func (w Word) Mp3() []string {
	return []string{
		w.Audio.Mp3,
	}
}

func (w Word) Pronunciation() string {
	return w.Audio.Syllables + " | " + fmt.Sprintf("/%v/", w.Audio.Pronunciation)
}

func (w Word) DefinitionHtml(showWord bool) string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="word-content">`)

	sb.WriteString(fmt.Sprintf(`<div class="dict-name">%v</div>`, w.Type().Name()))

	if showWord {
		sb.WriteString(`<div class="this-word">`)
		sb.WriteString(w.W)
		sb.WriteString(`</div>`)
	}

	for _, def := range w.Defs {
		sb.WriteString(def.Html())
	}

	sb.WriteString(`</div>`)
	return sb.String()
}

var _ dict.Word = Word{}

type Definition struct {
	PartOfSpeech    string
	DefinitionEntry []DefinitionEntry
}

func (d Definition) Html() string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="definitions">`)

	sb.WriteString(`<div class="pos">`)
	sb.WriteString(partOfSpeech(d.PartOfSpeech))
	sb.WriteString(`</div>`)

	for i, subDef := range d.DefinitionEntry {
		sb.WriteString(`<div class="def-entry">`)

		sb.WriteString(`<table class="table-align">`)
		sb.WriteString(`<tr class="table-align">`)

		sb.WriteString(`<td class="table-align">`)
		sb.WriteString(`<div class="serial-no">`)
		sb.WriteString(fmt.Sprintf("%v", i+1))
		sb.WriteString(`</div>`)
		sb.WriteString("</td>")

		sb.WriteString("<td>")
		sb.WriteString(subDef.Html())
		sb.WriteString("</td>")

		sb.WriteString("</tr>")
		sb.WriteString("</table>")

		sb.WriteString(`</div>`)
	}

	sb.WriteString("</div>")
	return sb.String()
}

type DefinitionEntry struct {
	PartOfSpeech       string
	SubDefinitionEntry []SubDefinition
}

func (d DefinitionEntry) Html() string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="sub-def-list">`)
	sb.WriteString(`<table class="table-align">`)
	if d.PartOfSpeech != "" {
		sb.WriteString(`<div class="pos">`)
		sb.WriteString(partOfSpeech(d.PartOfSpeech))
		sb.WriteString(`</div>`)
	}

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
	Syllables     string
	Pronunciation string
	Mp3           string
}

type Example struct {
	Text string
}

func (e Example) Html() string {
	return fmt.Sprintf(`<div class=use-example>// %v</div>`, e.Text)
}

type websterDict struct {
	httpClient *http.Client
}

func (webster *websterDict) Parse(wordJson []byte) (dict.Word, error) {
	var word Word
	err := json.Unmarshal(wordJson, &word)
	return word, err
}

func NewDict() *websterDict {
	return &websterDict{
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

func (webster *websterDict) Type() dict.Dictionary {
	return dict.Webster
}

func (webster *websterDict) Lookup(word string) (dict.Word, error) {
	col := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36"),
	)
	col.SetClient(webster.httpClient)

	out := Word{}

	col.OnHTML(".widget.more_defs", func(element *colly.HTMLElement) {
		element.DOM.Remove()
	})

	// pronunciation
	col.OnHTML(".col", func(element *colly.HTMLElement) {
		prs := element.DOM.Find("span.prs > span.pr").Text()
		if prs != "" && out.Audio.Pronunciation == "" {
			out.Audio.Pronunciation = prs
			// mp3
			mp3 := element.DOM.Find("span.prs > a.play-pron")
			// dataLang, _ := mp3.Attr("data-lang")
			dataFile, _ := mp3.Attr("data-file")
			dataDir, _ := mp3.Attr("data-dir")
			out.Audio.Mp3 = fmt.Sprintf("https://media.merriam-webster.com/audio/prons/en/us/mp3/%v/%v.mp3",
				dataDir,
				dataFile,
			)
		}
	})
	col.OnHTML(".word-syllables", func(element *colly.HTMLElement) {
		if out.Audio.Syllables == "" {
			out.Audio.Syllables = element.Text
		}
	})

	var dictionaryEntry = 0
	col.OnHTML(".row.entry-header", func(element *colly.HTMLElement) {
		if out.W == "" {
			out.W = element.DOM.Find(".hword").Text()
		}
		def := Definition{
			PartOfSpeech:    element.DOM.Find(".fl").Text(),
			DefinitionEntry: []DefinitionEntry{},
			//Raw:          domHtml(element.DOM),
		}
		dictionaryEntry += 1
		parent := element.DOM.Parent()
		parent.Find(fmt.Sprintf("#dictionary-entry-%v", dictionaryEntry)).Find(".vg").Each(func(i int, vg *goquery.Selection) {
			vd := vg.Find(".vd").Text()
			vg.Find(".sb").Each(func(i int, selection *goquery.Selection) {
				defEntry := DefinitionEntry{
					PartOfSpeech: vd,
				}
				for span := 0; span < 100; span++ {
					spanClass := selection.Find(fmt.Sprintf(".sb-%v", span))
					if spanClass.Text() == "" {
						break
					}
					letter := spanClass.Find(".letter").Text()
					dt := spanClass.Find(".dtText").Text()
					// example
					var examples []Example
					spanClass.Find(".mw_t_sp").Each(func(i int, selection *goquery.Selection) {
						examples = append(examples, Example{
							Text: selection.Text(),
						})
					})
					defEntry.SubDefinitionEntry = append(defEntry.SubDefinitionEntry, SubDefinition{
						Def:      letter + " " + dt,
						Examples: examples,
						//Raw: domHtml(spanClass),
					})
				}
				def.DefinitionEntry = append(def.DefinitionEntry, defEntry)
			})
		})
		out.Defs = append(out.Defs, def)
	})

	col.OnRequest(func(r *colly.Request) {
		r.Headers.Add("accept", "*/*")
	})

	urlEncodedWord := url.QueryEscape(word)
	searchUrl := fmt.Sprintf(
		"https://www.merriam-webster.com/dictionary/%v",
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
		return "vi."
	}
	return pos
}
