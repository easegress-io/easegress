package jsontool

import "testing"

func TestJSONTrimNull(t *testing.T) {
	originJSON := `{"a":2,"b":3,"c":null,"d":{"x":"aaa","y":null,"z":"4444"},"e":[1,2,null,4],"f":{},"g":[]}`
	want := `{"a":2,"b":3,"d":{"x":"aaa","z":"4444"},"e":[1,2,4],"f":{},"g":[]}`
	rst, err := TrimNull([]byte(originJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(rst)

	if want != got {
		t.Fatalf("want: %s\ngot : %s", want, got)
	}

}
