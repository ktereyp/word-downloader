package dictcn

import (
	"encoding/json"
	"testing"
)

func TestCollinsDict_Lookup(t *testing.T) {
	dict := NewDict()
	word, err := dict.Lookup("regret")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	buf, _ := json.MarshalIndent(word, "", " ")
	t.Logf("%v", string(buf))
}
