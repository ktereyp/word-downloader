package collins

import "testing"

func TestCollinsDict_Lookup(t *testing.T) {
	dict := NewDict()
	word, err := dict.Lookup("dexterous")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	t.Logf("%v", word)
}
