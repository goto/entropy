package firehose2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/goto/entropy/pkg/errors"
)

func mustJSON(v any) json.RawMessage {
	bytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes
}

func validateStruct(structVal any) error {
	err := validator.New().Struct(structVal)
	if err != nil {
		var fields []string
		for _, fieldError := range err.(validator.ValidationErrors) {
			fields = append(fields, fmt.Sprintf("%s: %s", fieldError.Field(), fieldError.Tag()))
		}
		return errors.ErrInvalid.
			WithMsgf("invalid values for fields").
			WithCausef(strings.Join(fields, ", "))
	}
	return nil
}
