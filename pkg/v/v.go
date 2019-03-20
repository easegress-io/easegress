package v

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"

	localesen "github.com/go-playground/locales/en"
	unitran "github.com/go-playground/universal-translator"
	validator "gopkg.in/go-playground/validator.v9"
	tranen "gopkg.in/go-playground/validator.v9/translations/en"
)

const (
	validatorTag           = "v"
	validateParentTagValue = "parent"
)

var (
	globalValidator  *validator.Validate
	globalTranslator unitran.Translator
)

type (
	// ValidateFunc is the type of validate func.
	ValidateFunc = validator.Func

	// Validator is the type which can be validated, it must be struct.
	// We will call its method Validate if the struct put v's tag parent on
	// a must string field as below.
	//
	// Spec struct {
	// 	V string `v:"parent"`
	// }
	Validator interface {
		Validate() error
	}
)

// Struct validates struct type value.
func Struct(v interface{}) error {
	err := globalValidator.Struct(v)
	if err == nil {
		return nil
	}

	// defensive programming
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	var errs []string
	for _, fe := range ve {
		var err string
		if fe.Tag() == "parent" {
			err = fe.Value().(string)
		} else {
			err = fe.Translate(globalTranslator)
		}
		errs = append(errs, err)
	}

	return fmt.Errorf("%s", strings.Join(errs, "\n"))
}

func init() {
	globalValidator = validator.New()

	enTran := localesen.New()
	uniTran := unitran.New(enTran)
	globalTranslator, _ = uniTran.GetTranslator("en")

	tranen.RegisterDefaultTranslations(globalValidator, globalTranslator)

	globalValidator.SetTagName(validatorTag)
	globalValidator.RegisterTagNameFunc(getYAMLName)

	Register("parent", parent, "" /*be translated in validateSpec*/)

	Register("urlname", urlName, "{0} '{1}' is an invalid url name")
	Register("httpmethod", httpMethod, "{0} '{1}' is an invalid http method")
	Register("httpcode", httpCode, "{0} '{1}' is an invalid http code")
	Register("prefix", prefix, "{0} '{1}' has not prefix '{2}'")
	Register("regexp", _regexp, "{0} '{1}' is an invalid regexp")
}

func getYAMLName(field reflect.StructField) string {
	name := strings.SplitN(field.Tag.Get("yaml"), ",", 2)[0]
	if name == "" {
		return field.Name
	}
	return name
}

// Register registers validate function, but we suggest put them in this package together.
func Register(name string, fn ValidateFunc, errText string) {
	globalValidator.RegisterValidation(name, fn)

	registerTran := func(uniTran unitran.Translator) error {
		return uniTran.Add(name, errText, true)
	}
	tran := func(uniTran unitran.Translator, fe validator.FieldError) string {
		t, _ := uniTran.T(name, fe.Field(),
			fmt.Sprintf("%v", fe.Value()),
			fe.Param())
		return t
	}

	globalValidator.RegisterTranslation(name, globalTranslator, registerTran, tran)
}

// parent validates the parent struct of the field, must be string which
// is used to propagate error message.
func parent(fl validator.FieldLevel) bool {
	v, ok := fl.Parent().Interface().(Validator)
	if !ok {
		logger.Errorf("BUG: %v is not Validator\n", fl.Parent().Type())
		return true
	}

	err := v.Validate()
	if err != nil {
		fl.Field().SetString(err.Error())
		return false
	}

	return true
}

func urlName(fl validator.FieldLevel) bool {
	return common.URL_FRIENDLY_CHARACTERS_REGEX.MatchString(fl.Field().String())
}

func httpMethod(fl validator.FieldLevel) bool {
	switch fl.Field().String() {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
}

func httpCode(fl validator.FieldLevel) bool {
	txt := http.StatusText(int(fl.Field().Int()))
	return len(txt) != 0
}

func prefix(fl validator.FieldLevel) bool {
	return strings.HasPrefix(fl.Field().String(), fl.Param())
}

func _regexp(fl validator.FieldLevel) bool {
	s := fl.Field().String()
	// Use omitempty if empty string is allowed.
	if len(s) == 0 {
		return false
	}
	_, err := regexp.Compile(s)
	return err == nil
}
