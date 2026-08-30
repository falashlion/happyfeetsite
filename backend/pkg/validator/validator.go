package validator

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var (
	v     *validator.Validate
	e164  = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
)

func init() {
	v = validator.New()
	_ = v.RegisterValidation("e164", func(fl validator.FieldLevel) bool {
		return e164.MatchString(fl.Field().String())
	})
}

// Decode decodes JSON from r.Body into dst and validates it.
func Decode(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return v.Struct(dst)
}

// Struct validates a struct.
func Struct(s any) error { return v.Struct(s) }
