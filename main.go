package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"word-downloader/dict"
	"word-downloader/dict/bingdict"
	"word-downloader/dict/collins"
	"word-downloader/dict/dictcn"
	"word-downloader/dict/webster"
)

var wordList = flag.String("word-list", "", "word list, if empty, read from stdio")
var dictionary = flag.String("dicts", "webster", "dictionary, comma separated. support: webster, dictcn, collins")
var sleepInterval = flag.Int64("sleep-interval", 1, "number of seconds to sleep before downloading next word")
var ankiCsv = flag.Bool("anki", false, "generate anki-flash csv file")
var downloadMp3 = flag.Bool("download-mp3", true, "whether download mp3")
var queryOnline = flag.Bool("query-online", true, "query online when missing")

var ankiDictScore = map[dict.Dictionary]int{
	dict.Collins:  0,
	dict.Webster:  1,
	dict.Dictcn:   2,
	dict.BingDict: 3,
}

func main() {
	flag.Parse()

	var myDicts []dict.Dict
	for _, dictName := range strings.Split(*dictionary, ",") {
		if dictName == "" {
			continue
		}
		switch dict.Dictionary(dictName) {
		case dict.Webster:
			myDicts = append(myDicts, webster.NewDict())
		case dict.BingDict:
			myDicts = append(myDicts, bingdict.NewBingDict())
		case dict.Dictcn:
			myDicts = append(myDicts, dictcn.NewDict())
		case dict.Collins:
			myDicts = append(myDicts, collins.NewDict())
		default:
			_, _ = fmt.Fprintf(os.Stderr, "unsuported dictionary: %v", *dictionary)
			flag.PrintDefaults()
			os.Exit(1)
		}
	}

	postAction := PostAction("")
	if *ankiCsv {
		postAction = AnCsv
	}

	var ankiFile *os.File
	if postAction == AnCsv {
		var err error
		ankiFile, err = os.Create("anki-flashcard.csv")
		if err != nil {
			log.Fatalf("error: cannot create anki-flashcard.csv: %v", err)
		}
	}
	defer ankiFile.Close()

	var wordSourceFile *os.File
	if *wordList == "" {
		wordSourceFile = os.Stdin
	} else {
		f, err := os.Open(*wordList)
		if err != nil {
			log.Fatalf("cannot open word list file: %v", err)
			return
		}
		wordSourceFile = f
		defer f.Close()
	}

	sort.Slice(myDicts, func(i, j int) bool {
		return ankiDictScore[myDicts[i].Type()] < ankiDictScore[myDicts[j].Type()]
	})
	var downloaders []*Downloader
	for _, dict := range myDicts {
		downloaders = append(downloaders, newDownloader(dict))
	}

	count := 0
	bufInput := bufio.NewReader(wordSourceFile)
	for {
		wordBytes, err := bufInput.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("error: failed to read word: %v", err)
			break
		}
		if len(wordBytes) > 0 {
			noWait := true
			var words []dict.Word
			for _, downloader := range downloaders {
				word, wordCached, err := downloader.download(strings.TrimSpace(string(wordBytes)))
				noWait = (noWait && wordCached) || err == dict.ErrNotFound
				if err == nil {
					words = append(words, word)
				}
			}

			// write to anki csv file
			if len(words) > 0 && ankiFile != nil {
				writeToAnkiCsv(ankiFile, words)
			}
			log.Printf("finish: %v", count)
			if !noWait {
				time.Sleep(time.Second * time.Duration(*sleepInterval))
			}
			count++
		}
		if err != nil {
			break
		}
	}
}

type Downloader struct {
	dict         dict.Dict
	audioDir     string
	picDir       string
	audioErrFile *os.File
	words        *os.File
	existWords   map[string]dict.Word
	//
	ankiFile *os.File
}

type PostAction string

const (
	AnCsv PostAction = "anki-csv"
)

func newDownloader(dict dict.Dict) *Downloader {
	myDictDir := string(dict.Type())
	err := os.MkdirAll(myDictDir, 0755)
	if err != nil {
		log.Fatalf("error: cannot mkdir: %v", err)
		return nil
	}
	downloader := &Downloader{
		dict:     dict,
		audioDir: filepath.Join(myDictDir, "audio"),
		picDir:   filepath.Join(myDictDir, "pic"),
	}

	err = os.MkdirAll(downloader.audioDir, 0755)
	if err != nil {
		log.Fatalf("error: cannot mkdir audio: %v", err)
		return nil
	}

	err = os.MkdirAll(downloader.picDir, 0755)
	if err != nil {
		log.Fatalf("error: cannot mkdir audio: %v", err)
		return nil
	}

	words, err := os.OpenFile(filepath.Join(myDictDir, "words.txt"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error: cannot create words.txt: %v", err)
	}
	downloader.words = words
	// read all finished
	downloader.loadAllFinished()

	errFile, err := os.OpenFile(filepath.Join(myDictDir, "audio-error.txt"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error: cannot create audio-error.txt: %v", err)
	}
	downloader.audioErrFile = errFile

	return downloader
}

func (d *Downloader) loadAllFinished() {
	d.existWords = map[string]dict.Word{}
	bufio := bufio.NewReader(d.words)
	var fileErr error
	for fileErr == nil {
		line, fileErr := bufio.ReadBytes('\n')
		if fileErr != nil && fileErr != io.EOF {
			log.Fatalf("error: cannot read from words.txt: %v", fileErr)
		}
		if len(line) == 0 {
			break
		}
		if strings.HasPrefix(string(line), "__not_found:") {
			word := strings.TrimPrefix(string(line), "__not_found:")
			word = strings.TrimSpace(word)
			d.existWords[word] = nil
			continue
		}
		word, err := d.dict.Parse(line)
		if err != nil {
			log.Fatalf("error: cannot unmarshal word from words.txt: %v, raw: %v", err, string(line))
		}
		d.existWords[word.Word()] = word
	}
}

func (d *Downloader) download(keyword string) (word dict.Word, cached bool, err error) {
	word, exist := d.existWords[keyword]
	if !exist {
		if !*queryOnline {
			return nil, true, dict.ErrNotFound
		}
		word, err = d.dict.Lookup(keyword)
		if err == dict.ErrNotFound {
			_, err = d.words.WriteString("__not_found:" + keyword + "\n")
			if err != nil {
				log.Fatalf("error: cannot write to disk: %v", err)
			}
			return nil, false, dict.ErrNotFound
		} else if err != nil {
			log.Printf("error: cannot query '%v': %v", keyword, err)
			return nil, false, err
		}
		log.Printf(" lookup ok: %v", word.Word())
	} else if word == nil {
		log.Printf(" lookup fail: %v [cache not found]", keyword)
		return nil, false, dict.ErrNotFound
	} else {
		log.Printf(" lookup ok: %v [cache]", word.Word())
	}
	cached = exist
	// write to disk
	if !exist {
		_, err = d.words.WriteString(word.Json() + "\n")
		if err != nil {
			log.Fatalf("error: cannot write to disk: %v", err)
		}
		d.existWords[keyword] = word
		d.existWords[word.Word()] = word
	}
	// download mp3/pic
	if *downloadMp3 {
		for _, mp3Url := range word.Mp3() {
			mp3Cached, err := d.downloadMp3(mp3Url)
			if err != nil {
				log.Printf("error: cannot download mp3 '%v': %v", mp3Url, err)
			}
			cached = cached && mp3Cached
		}
	}

	return word, cached, nil
}

func (d *Downloader) downloadMp3(url string) (cached bool, err error) {
	storeName := path.Base(url)
	return d.downloadFile(url, filepath.Join(d.audioDir, storeName))
}

func (d *Downloader) downloadPic(url string) (cached bool, err error) {
	storeName := path.Base(url)
	return d.downloadFile(url, filepath.Join(d.picDir, storeName))
}

func (d *Downloader) downloadFile(url string, storeName string) (cached bool, err error) {
	if url == "" {
		return false, nil
	}
	_, err = os.Stat(storeName)
	if err == nil {
		return true, nil
	}
	tmpFile := storeName + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return false, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return false, err
	}
	_ = f.Close()
	log.Printf(" download ok: %v", url)
	return false, os.Rename(tmpFile, storeName)
}

func writeToAnkiCsv(ankiFile *os.File, words []dict.Word) error {
	// word | pronunciation | example | word_html | sound

	var plainWord string

	defSb := strings.Builder{}
	defSb.WriteString(`<div class="background_card">`)
	var mp3 string
	var pronunciation string
	for _, word := range words {
		if plainWord == "" {
			plainWord = word.Word()
			pronunciation = fmt.Sprintf(`<div class="pronunciation">%v</div>`, word.Pronunciation())
			defSb.WriteString(`<div class="this-word">`)
			defSb.WriteString(plainWord)
			defSb.WriteString(`</div>`)
		}
		defSb.WriteString(fmt.Sprintf(`<div class="dict %v">%v</div>`, word.Type(), word.DefinitionHtml(false)))
		if word.Type() == dict.Webster {
			mp3List := word.Mp3()
			if len(mp3List) > 0 {
				mp3 = path.Base(mp3List[0])
			}
		}
	}
	defSb.WriteString(`</div>`)

	sb := strings.Builder{}
	sb.WriteString(plainWord)
	sb.WriteString("|")
	sb.WriteString(escapeVerticalBar(pronunciation))
	sb.WriteString("|")
	sb.WriteString("")
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf(`<div class="word">%v</div>`, escapeVerticalBar(defSb.String())))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf(`[sound:%v]`, mp3))
	sb.WriteString("\n")
	_, err := ankiFile.WriteString(sb.String())
	return err
}

func escapeVerticalBar(s string) string {
	return strings.ReplaceAll(s, "|", "%7C")
}
