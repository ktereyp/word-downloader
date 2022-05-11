package dict

import "fmt"

var ErrNotFound = fmt.Errorf("not found")

type Dictionary string

const (
	Webster  Dictionary = "webster"
	BingDict Dictionary = "bing-dict"
)

type Dict interface {
	Lookup(word string) (Word, error)
	Parse(wordJson []byte) (Word, error)
	Type() Dictionary
}

type Word interface {
	Word() string
	Pronunciation() string
	DefinitionHtml() string
	Json() string
	Type() Dictionary
	Mp3() []string
}
