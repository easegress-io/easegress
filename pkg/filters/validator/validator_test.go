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

package validator

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	cluster "github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/cluster/clustertest"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func createValidator(yamlConfig string, prev *Validator, supervisor *supervisor.Supervisor) *Validator {
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, err := filters.NewSpec(supervisor, "", rawSpec)
	if err != nil {
		panic(err.Error())
	}
	v := &Validator{spec: spec.(*Spec)}
	if prev == nil {
		v.Init()
	} else {
		v.Inherit(prev)
	}
	return v
}

func createClusterAndSyncer() (*clustertest.MockedCluster, chan map[string]string) {
	clusterInstance := clustertest.NewMockedCluster()
	syncer := clustertest.NewMockedSyncer()
	clusterInstance.MockedSyncer = func(t time.Duration) (cluster.Syncer, error) {
		return syncer, nil
	}
	syncerChannel := make(chan map[string]string)
	syncer.MockedSyncPrefix = func(prefix string) (<-chan map[string]string, error) {
		return syncerChannel, nil
	}
	return clusterInstance, syncerChannel
}

func TestHeaders(t *testing.T) {
	assert := assert.New(t)
	const yamlConfig = `
kind: Validator
name: validator
headers:
  Is-Valid:
    values: ["abc", "goodplan"]
    regexp: "^ok-.+$"
`

	v := createValidator(yamlConfig, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, req)

	result := v.Handle(ctx)
	assert.Equal(resultInvalid, result, "request has no header 'Is-Valid'")

	req.Header.Add("Is-Valid", "Invalid")
	result = v.Handle(ctx)
	assert.Equal(resultInvalid, result, "request has header 'Is-Valid', but value is incorrect")

	req.Header.Set("Is-Valid", "goodplan")
	result = v.Handle(ctx)
	assert.NotEqual(resultInvalid, result, "request has header 'Is-Valid' and value is correct")

	req.Header.Set("Is-Valid", "ok-1")
	result = v.Handle(ctx)
	assert.NotEqual(resultInvalid, result, "request has header 'Is-Valid' and matches the regular expression")
}

func TestJWTPublicKeySigning(t *testing.T) {
	type testCase struct {
		signName string
		//privateKey is for test validator here
		privateKey string
		yamlConf   string
		jwtToken   string
	}
	tests := []testCase{
		{
			signName:   "ECC",
			privateKey: "2d2d2d2d2d424547494e2045432050524956415445204b45592d2d2d2d2d0a4d494748416745414d424d4742797147534d34394167454743437147534d343941774548424730776177494241515167754a4f5a66454d7177376d78784e4f430a303477642f5a646a5a392b754165673048412b496f5734484963326852414e434141533765566272654e5338554f34547634637544454f6361413974794f78570a4f3772764c58593576514247346e70492f4e7a703355796c56522f5850366850616166624159397066444766464457447265424351746e560a2d2d2d2d2d454e442045432050524956415445204b45592d2d2d2d2d0a",
			jwtToken:   "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2NjQ0MzE4NDgsIm5hbWUiOiJKb2huIERvZSIsInN1YiI6IjEyMzQ1Njc4OTAifQ.6gi74rjAlo0rz_zOSBW3GYp1azekYurbjltzDh3xCYaJT-Vk7jUOOGedU5X7mGrv1wtvO3JCc5L40GSqscdM5w",
			yamlConf: `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: ES256
  publicKey: "2d2d2d2d2d424547494e205055424c4943204b45592d2d2d2d2d0a4d466b77457759484b6f5a497a6a3043415159494b6f5a497a6a3044415163445167414575336c5736336a557646447545372b484c6778446e47675062636a730a566a7536377931324f62304152754a3653507a633664314d70565566317a2b6f54326d6e32774750615877786e78513167363367516b4c5a31513d3d0a2d2d2d2d2d454e44205055424c4943204b45592d2d2d2d2d0a"
`}, {
			signName:   "RSA",
			privateKey: "2d2d2d2d2d424547494e205253412050524956415445204b45592d2d2d2d2d0a4d494945766749424144414e42676b71686b6947397730424151454641415343424b67776767536b41674541416f494241514447466d63535853674a546364320a4167336d4b6d6f5a73553448577373354b707567517965545263443972756430386963505748556543517257336952646a2f55667178744c4c327641594562740a37545237784575774b495936416875696d6966552f743961386f59444c7347706f76584b6f496b647a6d6a5547364e6c67396d527850677642713949707842590a5259326d596b56614a4c6d724664436b306a643235423768792f666f70476833796861314c354844486b77746d71394f4251636a7836796f49696c62535047640a562b436e6e526c6e634f42797952586930614249345365784d59636e48774f6b30465538536c5a515a6c3247504868652f457a347977764461504777597a65680a6b2b32365a526c5a3662497a5551427658726e43744b7835317832676e74362b76316c505268337775386d4d6279554b3832364153456d4964326333757753530a497251677042593541674d42414145436767454143674e393647612f4c4745374d524c2f6775416f427535347046534a7133556b387541534d7861326e39786b0a7050764d7a374449457547674936614e4c68476c385a6a6a77315139585464417671786346396d6666654d2b6a64596e63587662675a2f30794a4d30425273710a2f526c5931597079424169344d656a48784d7a3668617a77597567796d6a69663065614b4e3577474a3331747957464c37396b557072543366724368387165750a46782f4b762f4e6a366677376e4843676c38797033446b3178516d68752b786e68326874622f67746e5a31554d364d4d6a64396631455a484b4d4f65303330650a43546247756f694e2b6d542f57684744326e704a464f4e62312f6d514744466d305a456f2b5a324a436e2f424375755455524553375247305742626a59374d330a783857465a77636b6e4159382f75667051444e597135374d422b4247534a7644516f4c7965694b7655514b4267514449494e654d6662586e39782b33493666630a4e7959653774666f77506c717073456959796f7a5a4c582b5765526274394545564345445045522b3754413969624c45717761526f6268465545495a4e5a4b6d0a6c34617538446d2f7635673274506c382b4a2f6b42385841764462724d474537775569684e7264505156635139696446756661415959524b7454354f6e7953670a4855417243753152553673774b414d7879527577504a633850514b4267514439593754505554496531587a4d4b4a745a30553644594675504c3079786d6e71330a4e6472477a72646955436b394c674732646551744c2f357a686233497438625951616f39356f476c4e67427575324b6f6d4254556b746c7062665273706458550a732b31746a4c685530376f5469516771384175392b55657130537568314e6562344f684969783369496743627263662b376d31454148776c57365241346b646e0a5731304534354c3172514b426742565a493455794638694233526b394c58665a546a43346938476862446e4c52676a3043526c6f59643262477a674a654c74380a6566554e5a6355676169663257324b4e562b734c464577596a71522f7959414a342b30665a526d6d52346432634c4b374674744e5650514658397067303835370a424e4e6c736449376878306846506c6b4a2f357a364a664c6b3754785677665a6476486766595a535a593243687979315a6b5737674f7146416f4742414e4f470a53576b4c7568426458576d387544725a62485a6c6d4f6c46726674524877495556676243682f6e744f772f5565522b4d2b4b62304f72444c513676734a6e56660a4537504b326731467345532f37744d5936634b7575416d332f5751355a2f44424a774864682f39674a435373727748524536784b445a612b4f484e4844356f540a7654545a315639787a526f6f6a787a30676f685338302f6f57597a456d4b44696478746573733664416f4742414a63345a5a613562616a4372523768623144760a64784167375973775479693763763557334963653538394b4539704a537271654d72712b457a39566e474c713755323576766c2f386b473056534e444e4b64550a585671503743706576476151766c356c41304d63745868317136743739584b74476c7951754c75636370736b4d33484e774652504f50416f49586632433564660a4e55682f6f2f37543741655762546c4d506e4435673252460a2d2d2d2d2d454e44205253412050524956415445204b45592d2d2d2d2d0a",
			jwtToken:   "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2NjQ0MzM4OTQsIm5hbWUiOiJKb2huIERvZSIsInN1YiI6IjEyMzQ1Njc4OTAifQ.eWdiPudZrCjZbJeZ5sDbzzSpgoYwE5Z2deD0oKY5keSQ8-3ye8x3UXWnL5mh2kF7nm6TfO2TOsZh6mrlWV2uqkIi54gXbEbHQeg3sqQFRnUddeKecqQVlelDu7gKvLKngZcSnfcLshyN_GPS8X3XyGdxQq_K2g59bazqEHOsnnCNLKUvntLKtcB-63O91PY9v4SU93_Z6toiJKdO9hF-5NGj0UwcpIhZuL53lftxC6it-u0JlZUa3JuGGGGNqFTmKnn65OTnq-9VlXrov7na4LxBffL2d4h72EvB85DIU9zCavkyiNK0TDu2Uudf3PA1KCxOAWJXZ4ImH7m3gdWtvA",
			yamlConf: `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: RS256
  publicKey: "2d2d2d2d2d424547494e205055424c4943204b45592d2d2d2d2d0a4d494942496a414e42676b71686b6947397730424151454641414f43415138414d49494243674b434151454178685a6e456c306f435533486467494e356970710a4762464f4231724c4f5371626f454d6e6b3058412f61376e6450496e4431683148676b4b3174346b58592f31483673625379397277474247376530306538524c0a734369474f6749626f706f6e3150376657764b474179374271614c317971434a4863356f3142756a5a59505a6b6354344c776176534b63515745574e706d4a460a5769533571785851704e49336475516534637633364b526f64386f5774532b527778354d4c5a717654675548493865737143497057306a786e5666677035305a0a5a33446763736b5634744767534f456e735447484a783844704e42565045705755475a64686a78345876784d2b4d734c77326a7873474d336f5a5074756d555a0a57656d794d3145416231363577725373656463646f4a37657672395a54305964384c764a6a47386c43764e756745684a6948646e4e3773456b694b30494b51570a4f514944415141420a2d2d2d2d2d454e44205055424c4943204b45592d2d2d2d2d0a"
`}, {
			signName:   "EdDSA",
			privateKey: "338855991d10a976c7fd935d030135d194d660f62b5759e21d54cd0ecb66b1f6c7b07b59e3e52733f3a09471f7cd1ec797a1b27d8d57d31fce9bf07e389c10d5",
			jwtToken:   "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE2NjQ0MzQ5NDcsIm5hbWUiOiJKb2huIERvZSIsInN1YiI6IjEyMzQ1Njc4OTAifQ.lH_965QPuzJ28pwO9kWfGp7WpD89JpmO9pjoXBissGBtnUsoBvM3OIf8I18w6MhVF71bGW0qQoeCqrYW3gRaAA",
			yamlConf: `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: EdDSA
  publicKey: "2d2d2d2d2d424547494e205055424c4943204b45592d2d2d2d2d0a4d436f77425159444b32567741794541783742375765506c4a7a507a6f4a52783938306578356568736e324e56394d667a707677666a6963454e553d0a2d2d2d2d2d454e44205055424c4943204b45592d2d2d2d2d0a"
`}}
	assert := assert.New(t)
	for _, tc := range tests {
		v := createValidator(tc.yamlConf, nil, nil)

		ctx := context.New(nil)

		req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
		assert.Nil(err)
		setRequest(t, ctx, req)

		req.Header.Set("Authorization", "Bearer "+tc.jwtToken)
		result := v.Handle(ctx)
		if !assert.Equal(result, "") {
			t.Errorf("the jwt token in header should be valid")
		}

		req.Header.Set("Authorization", "Bearer "+tc.jwtToken+"abc")
		result = v.Handle(ctx)
		if !assert.Equal(result, resultInvalid) {
			t.Errorf("the jwt token in header should be invalid")
		}
	}
}

func TestJWTHMAC(t *testing.T) {
	assert := assert.New(t)
	const yamlConfig = `
kind: Validator
name: validator
jwt:
  cookieName: auth
  algorithm: HS256
  secret: "313233343536"
`
	v := createValidator(yamlConfig, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, req)

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	req.Header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.keH6T3x1z7mmhKL1T3r9sQdAxxdzB6siemGMr_6ZOwU"
	req.Header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in header should be valid")
	}

	req.Header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	req.Header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("the jwt token in header should be invalid")
	}

	req.Header.Del("Authorization")
	req.AddCookie(&http.Cookie{Name: "auth", Value: token})
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in cookie should be valid")
	}

	v = createValidator(yamlConfig, v, nil)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("the jwt token in cookie should be valid")
	}

	if v.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
}

func TestOAuth2JWT(t *testing.T) {
	assert := assert.New(t)

	const yamlConfig = `
kind: Validator
name: validator
oauth2:
  jwt:
    algorithm: HS256
    secret: "313233343536"
`
	v := createValidator(yamlConfig, nil, nil)

	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, req)

	token := "eyJhbGciOiJIUzM4NCIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.3Ywq9NlR3cBST4nfcdbR-fcZ8374RHzU50X6flKvG-tnWFMalMaHRm3cMpXs1NrZ"
	req.Header.Set("Authorization", "Bearer "+token)
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	req.Header.Set("Authorization", "Bearer "+token)
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("OAuth/2 Authorization should succeed")
	}

	req.Header.Set("Authorization", "not Bearer "+token)
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	req.Header.Set("Authorization", "Bearer "+token+"abc")
	result = v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func TestOAuth2TokenIntrospect(t *testing.T) {
	assert := assert.New(t)
	yamlConfig := `
kind: Validator
name: validator
oauth2:
  tokenIntrospect:
    endPoint: http://oauth2.megaease.com/
    insecureTls: true
    clientId: megaease
    clientSecret: secret
`
	v := createValidator(yamlConfig, nil, nil)
	ctx := context.New(nil)

	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(err)
	setRequest(t, ctx, req)

	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJzY29wZSI6Im1lZ2FlYXNlIn0.HRcRwN6zLJnubaUnZhZ5jC-j-rRiT-5mY8emJW6h6so"
	req.Header.Set("Authorization", "Bearer "+token)

	body := `{
			"subject":"megaease.com",
			"scope":"read,write",
			"active": false
		}`
	fnSendRequest = func(client *http.Client, r *http.Request) (*http.Response, error) {
		reader := strings.NewReader(body)
		return &http.Response{
			Body: io.NopCloser(reader),
		}, nil
	}
	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}

	yamlConfig = `
kind: Validator
name: validator
oauth2:
  tokenIntrospect:
    endPoint: http://oauth2.megaease.com/
    clientId: megaease
    clientSecret: secret
    basicAuth: megaease@megaease
`
	v = createValidator(yamlConfig, nil, nil)

	body = `{
			"subject":"megaease.com",
			"scope":"read,write",
			"active": true
		}`
	result = v.Handle(ctx)
	if result == resultInvalid {
		t.Errorf("OAuth/2 Authorization should succeed")
	}
}

func TestSignature(t *testing.T) {
	// This test is almost covered by signer

	const yamlConfig = `
kind: Validator
name: validator
signature:
  accessKeys:
    AKID: SECRET
`
	v := createValidator(yamlConfig, nil, nil)

	ctx := context.New(nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	assert.Nil(t, err)
	setRequest(t, ctx, req)

	result := v.Handle(ctx)
	if result != resultInvalid {
		t.Errorf("OAuth/2 Authorization should fail")
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func prepareCtxAndHeader() (*context.Context, http.Header) {
	ctx := context.New(nil)
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		panic(err)
	}
	setRequest(nil, ctx, req)
	return ctx, req.Header
}

func cleanFile(userFile *os.File) {
	err := userFile.Truncate(0)
	check(err)
	_, err = userFile.Seek(0, 0)
	check(err)
	userFile.Write([]byte(""))
}

func TestBasicAuth(t *testing.T) {
	userIds := []string{
		"userY", "userZ", "nonExistingUser",
	}
	passwords := []string{
		"userpasswordY", "userpasswordZ", "userpasswordX",
	}
	encrypt := func(pw string) string {
		encPw, err := bcryptHash([]byte(pw))
		check(err)
		return encPw
	}
	encryptedPasswords := []string{
		encrypt("userpasswordY"), encrypt("userpasswordZ"), encrypt("userpasswordX"),
	}

	t.Run("unexisting userFile", func(t *testing.T) {
		yamlConfig := `
kind: Validator
name: validator
basicAuth:
  mode: FILE
  userFile: unexisting-file`
		v := createValidator(yamlConfig, nil, nil)
		ctx, _ := prepareCtxAndHeader()
		if v.Handle(ctx) != resultInvalid {
			t.Errorf("should be invalid")
		}
	})
	t.Run("credentials from userFile", func(t *testing.T) {
		userFile, err := os.CreateTemp("", "apache2-htpasswd")
		check(err)

		yamlConfig := `
kind: Validator
name: validator
basicAuth:
  mode: FILE
  userFile: ` + userFile.Name()

		// test invalid format
		userFile.Write([]byte("keypass"))
		v := createValidator(yamlConfig, nil, nil)
		ctx, _ := prepareCtxAndHeader()
		if v.Handle(ctx) != resultInvalid {
			t.Errorf("should be invalid")
		}

		// now proper format
		cleanFile(userFile)
		userFile.Write(
			[]byte(userIds[0] + ":" + encryptedPasswords[0] + "\n" + userIds[1] + ":" + encryptedPasswords[1]))
		expectedValid := []bool{true, true, false}

		v = createValidator(yamlConfig, nil, nil)

		t.Run("invalid headers", func(t *testing.T) {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0])) // missing : and pw
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			assert.Equal(t, result, resultInvalid)
		})

		for i := 0; i < 3; i++ {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[i] + ":" + passwords[i]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			assert.Equal(t, expectedValid[i], result != resultInvalid)
		}

		cleanFile(userFile) // no more authorized users

		tryCount := 5
		for i := 0; i <= tryCount; i++ {
			time.Sleep(200 * time.Millisecond) // wait that cache item gets deleted
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0] + ":" + passwords[0]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			if result == resultInvalid {
				break // successfully unauthorized
			}
			if i == tryCount && result != resultInvalid {
				t.Errorf("should be unauthorized")
			}
		}

		os.Remove(userFile.Name())
		v.Close()
	})

	t.Run("test kvsToReader", func(t *testing.T) {
		kvs := make(map[string]string)
		kvs["/creds/key1"] = "key: key1\npass: pw"     // invalid
		kvs["/creds/key2"] = "ky: key2\npassword: pw"  // invalid
		kvs["/creds/key3"] = "key: key3\npassword: pw" // valid
		reader := kvsToReader(kvs)
		b, err := io.ReadAll(reader)
		check(err)
		s := string(b)
		assert.Equal(t, "key3:pw", s)
	})

	t.Run("dummy etcd", func(t *testing.T) {
		userCache := &etcdUserCache{prefix: ""} // should now skip cluster ops
		userCache.WatchChanges()
		assert.False(t, userCache.Match("doge", "dogepw"))
		userCache.Close()
	})

	t.Run("credentials from etcd", func(t *testing.T) {
		assert := assert.New(t)
		clusterInstance, syncerChannel := createClusterAndSyncer()

		// Test newEtcdUserCache
		if euc := newEtcdUserCache(clusterInstance, ""); euc.prefix != "/custom-data/credentials/" {
			t.Errorf("newEtcdUserCache failed")
		}
		if euc := newEtcdUserCache(clusterInstance, "/extra-slash/"); euc.prefix != "/custom-data/extra-slash/" {
			t.Errorf("newEtcdUserCache failed")
		}

		pwToYaml := func(key string, user string, pw string) string {
			if user != "" {
				return fmt.Sprintf("username: %s\npassword: %s", user, pw)
			}
			return fmt.Sprintf("key: %s\npassword: %s", key, pw)
		}
		kvs := make(map[string]string)
		kvs["/custom-data/credentials/1"] = pwToYaml(userIds[0], "", encryptedPasswords[0])
		kvs["/custom-data/credentials/2"] = pwToYaml("", userIds[2], encryptedPasswords[2])
		clusterInstance.MockedGetPrefix = func(key string) (map[string]string, error) {
			return kvs, nil
		}

		supervisor := supervisor.NewMock(
			nil, clusterInstance, nil, nil, false, nil, nil)

		yamlConfig := `
kind: Validator
name: validator
basicAuth:
  mode: ETCD
  etcdPrefix: credentials/
`
		expectedValid := []bool{true, false, true}
		v := createValidator(yamlConfig, nil, supervisor)
		for i := 0; i < 3; i++ {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[i] + ":" + passwords[i]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			assert.Equal(expectedValid[i], result != resultInvalid)
		}

		// first user is not authorized anymore
		kvs = make(map[string]string)
		kvs["/custom-data/credentials/2"] = pwToYaml("", userIds[2], encryptedPasswords[2])
		kvs["/custom-data/credentials/doge"] = `
randomEntry1: 21
nestedEntry:
  key1: val1
password: doge
key: doge
lastEntry: "byebye"
`
		syncerChannel <- kvs
		time.Sleep(time.Millisecond * 100)

		ctx, header := prepareCtxAndHeader()
		b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[0] + ":" + passwords[0]))
		header.Set("Authorization", "Basic "+b64creds)
		result := v.Handle(ctx)
		assert.Equal(resultInvalid, result)

		ctx, header = prepareCtxAndHeader()
		b64creds = base64.StdEncoding.EncodeToString([]byte("doge:doge"))
		header.Set("Authorization", "Basic "+b64creds)
		result = v.Handle(ctx)
		assert.NotEqual(resultInvalid, result)
		assert.Equal("doge", header.Get("X-AUTH-USER"))
		v.Close()
	})

	t.Run("credentials from LDAP", func(t *testing.T) {
		assert := assert.New(t)

		yamlConfig := `
kind: Validator
name: validator
basicAuth:
  mode: LDAP
  ldap:
    host: localhost
    port: 3893
    baseDN: ou=superheros,dc=glauth,dc=com
    uid: cn
    skipTLS: true
`
		// mock
		fnAuthLDAP = func(luc *ldapUserCache, username, password string) bool {
			return true
		}

		v := createValidator(yamlConfig, nil, nil)
		for i := 0; i < 3; i++ {
			ctx, header := prepareCtxAndHeader()
			b64creds := base64.StdEncoding.EncodeToString([]byte(userIds[i] + ":" + passwords[i]))
			header.Set("Authorization", "Basic "+b64creds)
			result := v.Handle(ctx)
			assert.True(result != resultInvalid)
		}

		v.Close()
	})
}
