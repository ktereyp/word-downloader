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
	"strings"
	"time"
	"word-downloader/dict"
	"word-downloader/dict/bingdict"
	"word-downloader/dict/webster"
)

var wordList = flag.String("word-list", "", "word list, if empty, read from stdio")
var dictionary = flag.String("dict", "webster", "dictionary, support: webster")
var sleepInterval = flag.Int64("sleep-interval", 1, "number of seconds to sleep before downloading next word")
var ankiCsv = flag.Bool("anki", false, "generate anki-flash csv file")

func main() {
	flag.Parse()

	var myDict dict.Dict
	switch dict.Dictionary(*dictionary) {
	case dict.Webster:
		myDict = webster.NewDict()
	case dict.BingDict:
		myDict = bingdict.NewBingDict()
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unsuported dictionary: %v", *dictionary)
		flag.PrintDefaults()
		os.Exit(1)
	}

	postAction := PostAction("")
	if *ankiCsv {
		postAction = AnCsv
	}

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

	downloader := newDownloader(myDict, postAction)

	count := 0
	bufInput := bufio.NewReader(wordSourceFile)
	for {
		wordBytes, err := bufInput.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("error: failed to read word: %v", err)
			break
		}
		if len(wordBytes) > 0 {
			downloader.download(strings.TrimSpace(string(wordBytes)))
			log.Printf("finish: %v", count)

			time.Sleep(time.Second * time.Duration(*sleepInterval))
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

func newDownloader(dict dict.Dict, postAction PostAction) *Downloader {
	myDictDir := string(dict.Type())
	err := os.MkdirAll(myDictDir, 0755)
	if err != nil {
		log.Fatalf("error: cannot mkdir: %v", err)
		return nil
	}
	err = os.Chdir(myDictDir)
	if err != nil {
		log.Fatalf("error: cannot change dir: %v", err)
		return nil
	}
	downloader := &Downloader{
		dict:     dict,
		audioDir: "audio",
		picDir:   "pic",
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

	words, err := os.OpenFile("words.txt", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error: cannot create words.txt: %v", err)
	}
	downloader.words = words
	// read all finished
	downloader.loadAllFinished()

	errFile, err := os.OpenFile("audio-error.txt", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error: cannot create audio-error.txt: %v", err)
	}
	downloader.audioErrFile = errFile

	if postAction == AnCsv {
		ankiFile, err := os.Create("anki-flashcard.csv")
		if err != nil {
			log.Fatalf("error: cannot create anki-flashcard.csv: %v", err)
		}
		downloader.ankiFile = ankiFile
	}

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
		word, err := d.dict.Parse(line)
		if err != nil {
			log.Fatalf("error: cannot unmarshal word from words.txt: %v, raw: %v", err, string(line))
		}
		d.existWords[word.Word()] = word
	}
}

func (d *Downloader) download(keyword string) error {
	var err error
	word, exist := d.existWords[keyword]
	if !exist {
		word, err = d.dict.Lookup(keyword)
		if err != nil {
			log.Printf("error: cannot query '%v': %v", keyword, err)
			return err
		}
		log.Printf(" lookup ok: %v", word.Word())
	} else {
		log.Printf(" lookup ok: %v [cache]", word.Word())
	}
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
	for _, mp3Url := range word.Mp3() {
		err = d.downloadMp3(mp3Url)
		if err != nil {
			log.Printf("error: cannot download mp3 '%v': %v", mp3Url, err)
		}
	}

	if d.ankiFile != nil {
		var mp3File string
		mp3 := word.Mp3()
		if len(mp3) > 0 {
			mp3File = path.Base(mp3[0])
		}

		d.writeToAnkiCsv(word, mp3File)
	}

	return nil
}

func (d *Downloader) downloadMp3(url string) error {
	storeName := path.Base(url)
	return d.downloadFile(url, filepath.Join(d.audioDir, storeName))
}

func (d *Downloader) downloadPic(url string) error {
	storeName := path.Base(url)
	return d.downloadFile(url, filepath.Join(d.picDir, storeName))
}

func (d *Downloader) downloadFile(url string, storeName string) error {
	if url == "" {
		return nil
	}
	_, err := os.Stat(storeName)
	if err == nil {
		return nil
	}
	tmpFile := storeName + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()
	log.Printf(" download ok: %v", url)
	return os.Rename(tmpFile, storeName)
}

func (d *Downloader) writeToAnkiCsv(word dict.Word, mp3File string) error {
	// word | example | word_html | sound
	if d.ankiFile == nil {
		return nil
	}
	sb := strings.Builder{}
	sb.WriteString(word.Word())
	sb.WriteString("|")
	sb.WriteString("")
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf(`<div class="word">%v</div>`, word.DefinitionHtml()))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf(`[sound:%v]`, mp3File))
	sb.WriteString("\n")
	_, err := d.ankiFile.WriteString(sb.String())
	return err
}
