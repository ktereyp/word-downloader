package webster

import (
	"encoding/json"
	"testing"
)

func Test_a(t *testing.T) {
	dict := NewDict()
	word, err := dict.Lookup("dexterous")
	if err != nil {
		t.Fatalf("cannot lookup: %v", err)
	}

	buf, _ := json.MarshalIndent(word, "", " ")
	t.Logf("w: %v", string(buf))
}
