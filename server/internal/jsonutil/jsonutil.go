package jsonutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v2"
)

type DecodeError struct {
	Field   string
	Message string
}

func (e *DecodeError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("field %q: %s", e.Field, e.Message)
	}
	return e.Message
}

func Unmarshal(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return wrapDecodeErr(err)
	}
	return nil
}

func UnmarshalStored(data []byte, dst any) error {
	if err := json.Unmarshal(data, dst); err != nil {
		return wrapDecodeErr(err)
	}
	return nil
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func BindJSON(c *fiber.Ctx, dst any) error {
	return Unmarshal(c.Body(), dst)
}

func wrapDecodeErr(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return &DecodeError{Field: typeErr.Field, Message: "expected " + typeErr.Type.String()}
	}
	if errors.Is(err, io.EOF) {
		return &DecodeError{Message: "request body is empty"}
	}
	return &DecodeError{Message: "invalid JSON body"}
}
