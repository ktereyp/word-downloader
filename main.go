package main

import (
	"bufio"
	"word-downloader/dict"
	"word-downloader/dict/bingdict"
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
)

var wordList = flag.String("word-list", "", "word list, if empty, read from stdio")

func main() {
	flag.Parse()

	bingDict := bingdict.NewBingDict()

	err := os.MkdirAll("audio", 0755)
	if err != nil {
		log.Fatalf("error: cannot mkdir audio: %v", err)
		return
	}

	audioErrFile, err := os.OpenFile("audio-error.txt", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("error: cannot create audio-error.txt: %v", err)
		return
	}
	defer audioErrFile.Close()

	csvFile, err := os.Create("anki-flashcard.csv")
	if err != nil {
		log.Fatalf("cannot create anki-flashcard file: %v", err)
	}
	defer csvFile.Close()

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

	count := 0
	bufInput := bufio.NewReader(wordSourceFile)
	for {
		wordBytes, err := bufInput.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("error: failed to read word: %v", err)
			break
		}
		if len(wordBytes) > 0 {
			word, err := bingDict.Lookup(strings.TrimSpace(string(wordBytes)))
			if err != nil {
				log.Printf("look up error: %v", err)
				if err == dict.ErrNotFound {
					continue
				}
			}
			err = downloadMp3(word.Audio.USAudio)
			if err != nil {
				log.Printf("error: cannot download file '%v': %v", word.Audio.USAudio, err)
				audioErrFile.WriteString(word.Audio.USAudio + "\n")
			}

			record := wordToCsv(word)
			_, err = csvFile.WriteString(record + "\n")
			if err != nil {
				log.Printf("error: cannot write record: %v", err)
				return
			}

			time.Sleep(time.Second)
			count++
			log.Printf("finish: %v", count)
		}
		if err != nil {
			break
		}
	}
}

// word | example | word_html | sound
func wordToCsv(word dict.Word) string {
	var examples string
	for _, example := range word.Examples {
		examples += example.Raw
	}
	examples = strings.ReplaceAll(examples, "|", "%7C")
	examples = strings.ReplaceAll(examples, "\n", "")

	var defHtml string
	for _, def := range word.Defs {
		defHtml += fmt.Sprintf(`<div class="bing-dict">%v</div>`, def.Raw)
	}
	defHtml = strings.ReplaceAll(defHtml, "|", "%7C")
	defHtml = strings.ReplaceAll(defHtml, "\n", "")

	// download audio
	mp3 := fmt.Sprintf("[sound:%v]", path.Base(word.Audio.USAudio))

	return fmt.Sprintf("%v|%v|%v|%v",
		word.W,
		examples,
		defHtml,
		mp3,
	)
}

func downloadMp3(url string) error {
	if url == "" {
		return nil
	}
	filename := path.Base(url)
	target := filepath.Join("audio", filename)
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	tmpFile := target + ".tmp"
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
	return os.Rename(tmpFile, target)
}
