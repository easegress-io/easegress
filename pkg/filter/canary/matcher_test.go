package canary

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
)

const tempDir = "/tmp/eg-test"

func TestMain(m *testing.M) {
	absLogDir := filepath.Join(tempDir, "global-log")
	os.MkdirAll(absLogDir, 0755)
	logger.Init(&option.Options{
		Name:      "canary-for-log",
		AbsLogDir: absLogDir,
	})

	code := m.Run()

	logger.Sync()
	os.RemoveAll(tempDir)

	os.Exit(code)
}

func TestIsValidParenthesis(t *testing.T) {

	testCases := []struct {
		s   string
		exp bool
	}{
		{
			"",
			true,
		},
		{
			"(",
			false,
		},
		{
			")",
			false,
		},
		{
			"()",
			true,
		},
		{
			"(()",
			false,
		},
		{
			"())",
			false,
		},
		{
			"(())",
			true,
		},
		{
			"aaa",
			true,
		},
	}

	for i, c := range testCases {
		if isValidParenthesis(c.s) != c.exp {
			t.Errorf("case.%d, expected: %t, but got: %t",
				i, c.exp, !c.exp)
		}
	}
}

func TestParserCheck(t *testing.T) {

	testCases := []struct {
		conditions string
		pass       bool
	}{
		{
			"&&",
			false,
		},
		{
			"a == b))",
			false,
		},
		{
			"Header.B == b))",
			false,
		},
		{
			"Header.A == 'A'",
			true,
		},
	}

	for i, c := range testCases {
		p := &parser{
			conditions: c.conditions,
			offset:     0,
			step:       stepBegin,
		}
		err := p.check()
		if err != nil && c.pass {
			t.Errorf("case.%d, expected pass, but got: %s", i, err.Error())
		}
		if err == nil && !c.pass {
			t.Errorf("case.%d, expected failed, but pass", i)
		}
	}
}

func TestDoParseCondIllegalConditions(t *testing.T) {
	testCases := []struct {
		conditions string
		pass       bool
	}{
		{
			"XXX == ",
			false,
		},
		{
			"ClientIP == ",
			false,
		},
		{
			"ClientIP mod 'A'",
			false,
		},
		{
			"ClientIP mod '100'",
			true,
		},
		{
			"Header *A mod '1'",
			false,
		},
		{
			"ClientIP == 'A",
			false,
		},
		{
			"Header.A == '",
			false,
		},
	}

	for i, c := range testCases {
		_, err := makeMatcher(c.conditions)
		if err != nil && c.pass {
			t.Errorf("case.%d, expected pass, but got: %s", i, err.Error())
		}
		if err == nil && !c.pass {
			t.Errorf("case.%d, expected failed, but pass", i)
		}
	}
}

func TestMakeMatcherIllegalSource(t *testing.T) {

	testCases := []struct {
		conditions string
		pass       bool
	}{
		{
			"Header",
			false,
		},
		{
			"",
			false,
		},
		{
			"Header.A",
			false,
		},
		{
			"Header.A == ",
			false,
		},
	}

	for i, c := range testCases {
		_, err := makeMatcher(c.conditions)
		if err != nil && c.pass {
			t.Errorf("case.%d, expected pass, but got: %s", i, err.Error())
		}
		if err == nil && !c.pass {
			t.Errorf("case.%d, expected failed, but pass", i)
		}
	}
}

func TestMakeMatcherIllegalCondition(t *testing.T) {

	testCases := []struct {
		conditions string
		pass       bool
	}{
		{
			"Header.A == 'A' ** B",
			false,
		},
	}

	for i, c := range testCases {
		_, err := makeMatcher(c.conditions)
		if err != nil && c.pass {
			t.Errorf("case.%d, expected pass, but got: %s", i, err.Error())
		}
		if err == nil && !c.pass {
			t.Errorf("case.%d, expected failed, but pass", i)
		}
	}
}

func TestMatcherSources(t *testing.T) {

	req, err := http.NewRequest("GET", "http://", nil)
	if err != nil {
		t.Fatal(err)
	}

	data := &sourceData{
		req:      req,
		clientIP: "",
	}

	token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJmb28i" +
		"OiJiYXIifQ.FhkiHkoESI_cG3NPigFrxEk9Z60_oXrOT2vGm9Pn6R" +
		"DgYNovYORQmmA0zs1AoAOf09ly2Nx2YAg6ABqAYga1AcMFkJljwxTT" +
		"5fYphTuqpWdy4BELeSYJx5Ty2gmr8e7RonuUztrdD5WfPqLKMm1Ozp_" +
		"T6zALpRmwTIW0QPnaBXaQD90FplAg46Iy1UlDKr-Eupy0i5SLch5Q-p2" +
		"ZpaL_5fnTIUDlxC3pWhJTyx_71qDI-mAA_5lE_VdroOeflG56sSmDxopP" +
		"EG3bFlSu1eowyBfxtu0_CuVd-M42RU75Zc4Gsj6uV77MBtbMrf4_7M_NUT" +
		"SgoIF3fRqxrj0NzihIBg"

	testCases := []struct {
		conditions string
		dataAction func()
	}{
		{
			"Header.A == 'A'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Cookie.A == 'A'",
			func() {
				data.req.AddCookie(&http.Cookie{
					Name:  "A",
					Value: "A",
				})
			},
		},
		{
			"Jwt.foo == 'bar'",
			func() {
				data.req.Header.Set("Authorization", "Bearer "+token)
			},
		},
		{
			"ClientIP == 'A'",
			func() {
				data.clientIP = "A"
			},
		},
	}

	for i, c := range testCases {
		m, err := makeMatcher(c.conditions)
		if err != nil {
			t.Fatal(err)
		}

		c.dataAction()

		if !m(data) {
			t.Errorf("case.%d, expected match", i)
		}
	}
}

// Get Value from request failed.
func TestMatcherGetActVal(t *testing.T) {
	req, err := http.NewRequest("GET", "http://", nil)
	if err != nil {
		t.Fatal(err)
	}

	data := &sourceData{
		req:      req,
		clientIP: "",
	}

	illegalToken := "illegalToken"
	token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJmb28i" +
		"OiJiYXIifQ.FhkiHkoESI_cG3NPigFrxEk9Z60_oXrOT2vGm9Pn6R" +
		"DgYNovYORQmmA0zs1AoAOf09ly2Nx2YAg6ABqAYga1AcMFkJljwxTT" +
		"5fYphTuqpWdy4BELeSYJx5Ty2gmr8e7RonuUztrdD5WfPqLKMm1Ozp_" +
		"T6zALpRmwTIW0QPnaBXaQD90FplAg46Iy1UlDKr-Eupy0i5SLch5Q-p2" +
		"ZpaL_5fnTIUDlxC3pWhJTyx_71qDI-mAA_5lE_VdroOeflG56sSmDxopP" +
		"EG3bFlSu1eowyBfxtu0_CuVd-M42RU75Zc4Gsj6uV77MBtbMrf4_7M_NUT" +
		"SgoIF3fRqxrj0NzihIBg"

	testCases := []struct {
		conditions string
		dataAction func()
	}{
		{
			"Header.A == 'A'",
			func() {
				data.req.Header.Set("B", "A")
			},
		},
		{
			"Cookie.A == 'A'",
			func() {
				data.req.AddCookie(&http.Cookie{
					Name:  "B",
					Value: "A",
				})
			},
		},
		{
			"Jwt.foo == 'bar'",
			func() {},
		},
		{
			"Jwt.foo == 'bar'",
			func() {
				data.req.Header.Set("Authorization", "Bearer "+illegalToken)
			},
		},
		{
			"Jwt.foo == 'rab'",
			func() {
				data.req.Header.Set("Authorization", "Bearer "+token)
			},
		},
		{
			"ClientIP == 'A'",
			func() {
			},
		},
	}

	for i, c := range testCases {
		m, err := makeMatcher(c.conditions)
		if err != nil {
			t.Fatal(err)
		}

		c.dataAction()

		if m(data) {
			t.Errorf("case.%d, expected mismatch", i)
		}
	}
}

func TestMatcherConditionalOP(t *testing.T) {

	req, err := http.NewRequest("GET", "http://", nil)
	if err != nil {
		t.Fatal(err)
	}

	data := &sourceData{
		req:      req,
		clientIP: "",
	}

	testCases := []struct {
		conditions string
		dataAction func()
	}{
		{
			"Header.A == 'A'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Header.A != 'A'",
			func() {
				data.req.Header.Set("A", "B")
			},
		},
		{
			"Header.A > 'A'",
			func() {
				data.req.Header.Set("A", "B")
			},
		},
		{
			"Header.A >= 'A'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Header.A >= 'A'",
			func() {
				data.req.Header.Set("A", "B")
			},
		},
		{
			"Header.A < 'B'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Header.A <= 'B'",
			func() {
				data.req.Header.Set("A", "B")
			},
		},
		{
			"Header.A <= 'B'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Header.A in 'A, B'",
			func() {
				data.req.Header.Set("A", "A")
			},
		},
		{
			"Header.A in 'A, B'",
			func() {
				data.req.Header.Set("A", "B")
			},
		},
		{
			"Header.A mod '13'", // hash('A') mod 100 == 12
			func() {
				data.req.Header.Set("A", "A")
			},
		},
	}

	for i, c := range testCases {
		m, err := makeMatcher(c.conditions)
		if err != nil {
			t.Fatal(err)
		}

		c.dataAction()

		if !m(data) {
			t.Errorf("case.%d, expected match, conditions: %s", i, c.conditions)
		}
	}
}

func TestMatcherLogicalOP(t *testing.T) {

	req, err := http.NewRequest("GET", "http://", nil)
	if err != nil {
		t.Fatal(err)
	}

	data := &sourceData{
		req:      req,
		clientIP: "",
	}

	testCases := []struct {
		conditions string
		dataAction func()
		exp        bool
	}{
		{
			"Header.A == 'A' && Header.B == 'B'",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "B")
			},
			true,
		},
		{
			"Header.A == 'A' || Header.B == 'B'",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
			},
			true,
		},
		{
			"Header.A == 'A' && Header.B == 'B' || Header.C == 'C'",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
			},
			false,
		},
		{
			"Header.A == 'A' && Header.B == 'B' || Header.C == 'C'",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")
			},
			true,
		},
		{
			"Header.A == 'A' && (Header.B == 'B' || Header.C == 'C')",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Del("C")

			},
			false,
		},
		{
			"Header.A == 'A' && (Header.B == 'B' || Header.C == 'C')",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")

			},
			true,
		},
		{
			"Header.A == 'A' && (Header.B == 'B' || " +
				"(Header.C == 'C' && Header.D == 'D'))",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")

			},
			false,
		},
		{
			"Header.A == 'A' && (Header.B == 'B' || " +
				"(Header.C == 'C' && Header.D == 'D'))",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")
				data.req.Header.Set("D", "D")

			},
			true,
		},
		{
			"Header.A == 'A' && (Header.E == 'E' || Header.F == 'F') " +
				"&& (Header.B == 'B' || " +
				"(Header.C == 'C' && Header.D == 'D'))",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")
				data.req.Header.Del("D")
				data.req.Header.Set("E", "E")
			},
			false,
		},
		{
			"Header.A == 'A' && (Header.E == 'E' || Header.F == 'F') " +
				"&& (Header.B == 'B' || " +
				"(Header.C == 'C' && Header.D == 'D'))",
			func() {
				data.req.Header.Set("A", "A")
				data.req.Header.Set("B", "A")
				data.req.Header.Set("C", "C")
				data.req.Header.Set("D", "D")
				data.req.Header.Set("E", "E")

			},
			true,
		},
	}

	for i, c := range testCases {
		m, err := makeMatcher(c.conditions)
		if err != nil {
			t.Fatal(err)
		}

		c.dataAction()

		if m(data) != c.exp {
			t.Errorf("case.%d, expected: %t, conditions: %s", i, c.exp,
				c.conditions)
		}
	}
}

func BenchmarkMatcher(b *testing.B) {
	// Should short circuit (We can observe it in CPU profile).
	// It's perf should be almost as fast as c := "Header.A == 'A'.
	c := "Header.A == 'A' || Jwt.B != 'B' || Cookie.C > 'C'" +
		"|| Jwt.C != 'C' || Jwt.D != 'D' || Jwt.E != 'E' || Jwt.F != 'F'"
	//c := "Header.A == 'A'"
	m, err := makeMatcher(c)
	if err != nil {
		b.Fatal(err)
	}
	req, err := http.NewRequest("GET", "http://", nil)
	if err != nil {
		b.Fatal(err)
	}
	req.Header.Set("A", "A")
	sd := &sourceData{
		req:      req,
		clientIP: "",
	}
	for i := 0; i < b.N; i++ {
		m(sd)
	}
}
