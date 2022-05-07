package bingdict

import (
	"encoding/json"
	"testing"
)

func TestBingDict_Lookup(t *testing.T) {
	dict := NewBingDict()
	word, err := dict.Lookup("kestrel")
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := json.MarshalIndent(word, "", " ")
	t.Logf("%v", string(buf))
}
