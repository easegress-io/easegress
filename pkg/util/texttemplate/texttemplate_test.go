package texttemplate

import (
	"testing"
)

func TestNewTextTemplateSucc(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	})

	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if res := tt.MatchMetaTemplate("plugin.abc.req.body"); len(res) == 0 {
		t.Errorf("input %s match template %s", "plugin.abc.req.body", res)
	}

	if err = tt.SetDict("plugin.abc.req.body", "kkk"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if s, err := tt.Render("xxx-[[plugin.abc.req.body]]--yyy"); s != "xxx-kkk--yyy" || err != nil {
		t.Errorf("rendering fail , result is %s except xxx-kkk--yyy", s)
	}
}
func TestNewTextTemplateRenderGJSON(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
	})

	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err = tt.SetDict("plugin.abc.req.body", "{\"aaa\":\"bbb\", \"kkk\":\"hhh\", \"t\":{\"jjj\":\"qqq\"}}"); err != nil {
		t.Errorf("set failed err %v", err)
	}

	input := "001010-[[plugin.abc.req.body.aaa]]--02020"
	except := "001010-bbb--02020"

	if s, err := tt.Render(input); s != except || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, excpet %s , after rending %s ", input, except, s)
	}

	input = "001010-[[plugin.abc.req.body.kkk]]--02020"
	except = "001010-hhh--02020"

	if s, err := tt.Render(input); s != except || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, excpet %s , after rending %s ", input, except, s)

	}

	input = "001010-[[plugin.abc.req.body.t.jjj]]--02020"
	except = "001010-qqq--02020"

	if s, err := tt.Render(input); s != except || err != nil {
		t.Errorf("rendering failed err %v", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, excpet %s , after rending %s ", input, except, s)
	}
}

func TestNewTextTemplateRenderWildCard(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
	})

	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	if err := tt.SetDict("plugin.abc.req.header.X-forward", "a.b.com"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	if err = tt.SetDict("plugin.abc.req.header.Y-forward", "q.m.com"); err != nil {
		t.Errorf("set failed err =%v", err)
	}

	input := "001010-[[plugin.abc.req.header.X-forward]]--02020"
	except := "001010-a.b.com--02020"

	if s, err := tt.Render(input); s != except || err != nil {
		t.Errorf("rendering err is %v ", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, excpet %s , after rending %s ", input, except, s)
	}

	input = "001010-[[plugin.abc.req.header.Y-forward]]--02020"
	except = "001010-q.m.com--02020"

	if s, err := tt.Render(input); s != except || err != nil {
		t.Errorf("rendering err is %v ", err)
		t.Errorf("dict is %v ", tt.GetDict())
		t.Fatalf("input %s, excpet %s , after rending %s ", input, except, s)

	}

}
func TestNewTextTemplateErrGJSON(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.req.{gjson}.body",
	})

	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrGJSONBegin(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"{gjson}.plugin.{}.req.body",
	})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrWidecardConfilct(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.abc.req.header",
	})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateErrGJSONMiddle(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.{gjson}.header",
	})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithEmpty(t *testing.T) {

	tt, err := NewDefault([]string{})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithWidecarFirstLevel(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin",
		"name",
		"key",
		"{}",
	})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateWithWidecarLastLevel(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.req.http",
		"plugin.req.name",
		"plugin.req.url",
		"plugin.req.{}",
	})

	t.Logf("New engine invalid, excpet err [%v]", err)
	if err == nil {
		t.Fatalf("new engine should failed, but succ %v, tt %v", err, tt)
	}
}

func TestNewTextTemplateValidate(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	})

	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	input := "try to ...[[plugin.abc.req]]"
	if tt.HasTemplates(input) == false {
		t.Fatalf("except match template, but failed %s", input)
	}

	input = "try to ...plugin.abc.req"
	if tt.HasTemplates(input) == true {
		t.Fatalf("except not match template, but succ input [%s]", input)
	}

	input = "try to ...[[plugin.abc.tkg]]..."
	if tt.HasTemplates(input) == true {
		t.Fatalf("except not match template, but succ input [%s]", input)
	}

	input = "try to ...[[.]]..."
	if tt.HasTemplates(input) == true {
		t.Fatalf("except not match template, but succ input [%s]", input)
	}

}

func TestNewTextTemplateMatchTemplate(t *testing.T) {
	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.host.{}",
	})

	if err != nil {
		t.Errorf("new engine failed err %v", err)
	}

	input := "."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = ".."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = ""
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = "kkk"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = "[["
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = "]]"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}
	input = "{}"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = "{gjson}"
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but succ input [%s]", result)
	}

	input = "plugin.name.."
	if result := tt.MatchMetaTemplate(input); len(result) != 0 {
		t.Fatalf("except not match template, but input [%s]", result)
	}

	input = "plugin.name.req"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("except match template, but failed input [%s]", result)
	}

	input = "plugin.name.req.host"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("except match template, but failed input [%s]", result)
	}

	input = "plugin.name.req.host.abb"
	if result := tt.MatchMetaTemplate(input); len(result) == 0 {
		t.Fatalf("except match template, but failed input [%s]", result)
	}
}
func TestNewTextTemplateExtractTemplateRuleMap(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	})

	if err != nil {
		t.Fatalf("new engine failed err %v", err)
	}

	input := "xxx000[[plugin.abc.req.path]]xxx,yyy[[plugin.0.rsp.statuscode]],fff[[plugin.3.req.body.jjj.qqq.ddd]]"

	m := map[string]string{}
	if m = tt.ExtractTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s faile, nothing matched", input)
	}

	key1 := "plugin.abc.req.path"
	key2 := "plugin.0.rsp.statuscode"
	key3 := "plugin.3.req.body.jjj.qqq.ddd"
	value3 := "plugin.3.req.body.{gjson}"

	if k, exist := m[key1]; !exist {
		t.Errorf("extract from key1 :%s from input %s faile, nothing matched", key1, input)
	} else {
		if k != key1 {
			t.Errorf("extract from key1 :%s  value %s from input %s not matched", key1, k, input)
		}
	}

	if k, exist := m[key2]; !exist {
		t.Errorf("extract from key1 :%s from input %s faile, nothing matched", key2, input)
	} else {
		if k != key2 {
			t.Errorf("extract from key1 :%s  value %s from input %s not matched", key2, k, input)
		}
	}

	if k, exist := m[key3]; !exist {
		t.Errorf("extract from key3 :%s from input %s faile, nothing matched", key3, input)
	} else {
		if k != value3 {
			t.Errorf("extract from key3 :%s  value %s from input %s not matched expect %s", key3, k, input, value3)
		}
	}

	input = "[[plugin.abc.req.path]]xxx,yyy[[plugin.0.rsp.statuscode]],fff[[plugin.3.req.body.jjj.qqq.ddd]]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) == 0 {
		t.Fatalf("extract from input %s faile, nothing matched", input)
	}
}

func TestNewTextTemplateExtractTemplateRuleMapEmpty(t *testing.T) {

	tt, err := NewDefault([]string{
		"plugin.{}.req.path",
		"plugin.{}.req.method",
		"plugin.{}.req.body",
		"plugin.{}.req.scheme",
		"plugin.{}.req.proto",
		"plugin.{}.req.host",
		"plugin.{}.req.body.{gjson}",
		"plugin.{}.req.header.{}",
		"plugin.{}.rsp.statuscode",
		"plugin.{}.rsp.body.{gjson}",
	})

	if err != nil {
		t.Fatalf("new engine failed err %v", err)
	}

	input := "xxx00011111[["

	m := map[string]string{}
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match except, should be empty", input)
	}

	input = "[[plugin.abc.req[[]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match except, should be empty", input)
	}

	input = "xxxxx[[plugin.abc.req]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match except, should be empty", input)
	}

	input = "[[plugin.abc.req]"
	if m = tt.ExtractTemplateRuleMap(input); len(m) != 0 {
		t.Fatalf("extract from input %s no match except, should be empty", input)
	}
}
