package common

import (
	"testing"
)

func TestScanTokensNormally(t *testing.T) {
	ret, err := ScanTokens(`abcdef`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abcdef` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi`, false, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\{def\}ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{def}ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	visitor := func(pos int, token string) (bool, string) {
		if pos != 13 {
			t.Fatalf("wrong token position: %d", pos)
		}
		if token != `jkl` {
			t.Fatalf("wrong token: %s", token)
		}

		return true, `JKL`
	}

	ret, err = ScanTokens(`abc\{def\}ghi{jkl}`, true, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{def}ghiJKL` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi{jkl}`, false, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\{def\}ghiJKL` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi{jkl}lmn`, true, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{def}ghiJKLlmn` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi{jkl}\{lmn\}`, false, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\{def\}ghiJKL\{lmn\}` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	// escape char out of token

	ret, err = ScanTokens(`abc\{def`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def`, false, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\{def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\{ghi`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{def{ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\}def`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc}def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\}def`, false, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\}def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\}def\}ghi\{`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc}def}ghi{` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	// escape char in token

	ret, err = ScanTokens(`abc\{defghi{\{jkl\{}`, true, nil)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{defghi{{jkl{}` {

		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	visitor = func(pos int, token string) (bool, string) {
		if pos != 11 {
			t.Fatalf("wrong token position: %d", pos)
		}
		if token != `{jkl{` {
			t.Fatalf("wrong token: %s", token)
		}

		return true, `\{JKL\}`
	}

	ret, err = ScanTokens(`abc\{defghi{\{jkl\{}`, true, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc{defghi{JKL}` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{defghi{\{jkl\{}`, false, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `abc\{defghi\{JKL\}` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	visitor = func(_ int, token string) (bool, string) {
		if token != `hello` {
			t.Fatalf("wrong token: %s", token)
		}

		return true, `world`
	}

	ret, err = ScanTokens(`\{hello\} - {hello}`, true, visitor)
	if err != nil {
		t.Fatalf("unexpected error")
	}
	if ret != `{hello} - world` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}
}

func TestScanTokensExceptionally(t *testing.T) {
	ret, err := ScanTokens(`abc{def`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc{def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc}def`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc}def` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def{ghi`, false, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc\{def{ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def}ghi`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc\{def}ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc}def}ghi`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc}def}ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc{def{ghi`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc{def{ghi` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\}def\{ghi{`, false, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc\}def\{ghi{` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}ghi}`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc\{def\}ghi}` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}

	ret, err = ScanTokens(`abc\{def\}{\{ghi\{{jkl}}`, true, nil)
	if err == nil {
		t.Fatalf("expected error unraied")
	}
	if ret != `abc\{def\}{\{ghi\{{jkl}}` {
		t.Fatalf("scan token returns wrong result: %s", ret)
	}
}
