package collins

import (
	"encoding/json"
	"fmt"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"word-downloader/dict"
)

type Word struct {
	W    string
	Defs []Definition
}

func (w Word) Word() string {
	return w.W
}

func (w Word) Json() string {
	buf, _ := json.Marshal(w)
	return string(buf)
}

func (w Word) Type() dict.Dictionary {
	return dict.Collins
}

func (w Word) Mp3() []string {
	return []string{}
}

func (w Word) Pronunciation() string {
	return ""
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

	if len(w.Defs) > 1 {
		for i, def := range w.Defs {
			sb.WriteString(def.Html(i + 1))
		}
	} else if len(w.Defs) == 1 {
		sb.WriteString(w.Defs[0].Html(0))
	}

	sb.WriteString(`</div>`)
	return sb.String()
}

var _ dict.Word = Word{}

type Definition struct {
	PartOfSpeech string
	Def          string
	Examples     []Example
}

func (d Definition) Html(serialNo int) string {
	sb := strings.Builder{}
	sb.WriteString(`<div class="definitions">`)

	sb.WriteString(`<div class="pos">`)
	if serialNo > 0 {
		sb.WriteString(fmt.Sprintf("%v. ", serialNo) + partOfSpeech(d.PartOfSpeech))
	} else {
		sb.WriteString(partOfSpeech(d.PartOfSpeech))
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`<div class="collins-def">`)
	sb.WriteString(d.Def)
	sb.WriteString(`</div>`)

	for _, example := range d.Examples {
		sb.WriteString(`<div class="collins-use-examples">`)
		sb.WriteString(example.Html())
		sb.WriteString("</div>")
	}

	sb.WriteString("</div>")
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

type collinsDict struct {
	service *selenium.Service
	wd      selenium.WebDriver
}

func (collins *collinsDict) Parse(wordJson []byte) (dict.Word, error) {
	var word Word
	err := json.Unmarshal(wordJson, &word)
	return word, err
}

func NewDict() *collinsDict {
	os.MkdirAll(filepath.Join(string(dict.Collins), "pages"), 0755)

	const (
		seleniumPath     = "selenium/selenium-server.jar"
		chromeDriverPath = "selenium/chromedriver"
		port             = 8080
	)
	opts := []selenium.ServiceOption{
		selenium.ChromeDriver(chromeDriverPath), // Specify the path to GeckoDriver in order to use Firefox.
		//selenium.Output(os.Stderr),              // Output debug information to STDERR.
	}
	service, err := selenium.NewSeleniumService(seleniumPath, port, opts...)
	if err != nil {
		panic(err) // panic is used only as an example and is not otherwise recommended.
	}

	// Connect to the WebDriver instance running locally.
	//caps := selenium.Capabilities{"browserName": "firefox"}
	caps := selenium.Capabilities{"browserName": "chrome"}
	caps.AddChrome(chrome.Capabilities{
		Path: "selenium/chrome-linux/chrome",
	})
	caps.AddProxy(selenium.Proxy{
		Type: selenium.Manual,
		HTTP: "http://127.0.0.1:8888",
		SSL:  "http://127.0.0.1:8888",
	})
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		panic(err)
	}

	return &collinsDict{
		service: service,
		wd:      wd,
	}
}

func (collins *collinsDict) Type() dict.Dictionary {
	return dict.Collins
}

func (collins *collinsDict) Lookup(word string) (dict.Word, error) {
	if err := collins.wd.Get("https://www.collinsdictionary.com/dictionary/english/" + word); err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%v__1", strings.ToLower(word))
	content, err := collins.wd.FindElement(selenium.ByID, id)
	if err != nil {
		return nil, dict.ErrNotFound
	}

	elements, err := content.FindElements(selenium.ByClassName, "hom")
	if err != nil {
		return nil, err
	}
	out := Word{}
	out.W = word
	for _, element := range elements {
		def := Definition{}
		posElement, err := element.FindElement(selenium.ByClassName, "pos")
		if err == nil {
			def.PartOfSpeech, _ = posElement.Text()
		}

		defElement, err := element.FindElement(selenium.ByClassName, "def")
		if err == nil {
			def.Def, _ = defElement.Text()
		}
		examples, err := element.FindElements(selenium.ByClassName, "type-example")
		for _, e := range examples {
			exampleStr, _ := e.Text()
			if exampleStr != "" {
				def.Examples = append(def.Examples, Example{Text: exampleStr})
			}
		}
		out.Defs = append(out.Defs, def)
	}

	pageSource, err := collins.wd.PageSource()
	if err == nil {
		ioutil.WriteFile(filepath.Join(string(dict.Collins), "pages", word+".html"), []byte(pageSource), 0644)
	}

	return out, nil
}

func partOfSpeech(pos string) string {
	return strings.ToLower(pos)
}
