package dict

import "fmt"

var ErrNotFound = fmt.Errorf("not found")

type Dictionary string

const (
	Webster  Dictionary = "webster"
	BingDict Dictionary = "bing-dict"
	Collins  Dictionary = "collins"
	Dictcn   Dictionary = "dictcn"
)

func (d Dictionary) Name() string {
	switch d {
	case Webster:
		return "Merriam-Webster"
	case Dictcn:
		return "海词"
	case BingDict:
		return "必应词典"
	case Collins:
		return "Collins"
	default:
		return string(d)
	}
}

type Dict interface {
	Lookup(word string) (Word, error)
	Parse(wordJson []byte) (Word, error)
	Type() Dictionary
}

type Word interface {
	Word() string
	Pronunciation() string
	DefinitionHtml(showWord bool) string
	Json() string
	Type() Dictionary
	Mp3() []string
}
