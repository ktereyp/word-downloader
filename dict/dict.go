package dict

import "fmt"

var ErrNotFound = fmt.Errorf("not found")

type Dict interface {
	Lookup(word string) (Word, error)
}

type Word struct {
	W        string
	Defs     []Definition
	Audio    Audio
	Pictures []string
	Examples []Example
}

type Definition struct {
	Kind string
	Def  string
	Raw  string
}

type Audio struct {
	PronunciationUS string
	USAudio         string
	PronunciationUK string
	UKAudio         string
}

type Example struct {
	Text string
	Raw  string
}
