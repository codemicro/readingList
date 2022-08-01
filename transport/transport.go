package transport

import "github.com/go-playground/validator"

var validate = validator.New()

// Inputs represents the data that is expected to be provided when adding a new
// entry to the reading list.
// The `query` struct tags are so this struct can be used in Fiber
// (github.com/gofiber/fiber/v2) applications and its (*ctx).QueryParser method
type Inputs struct {
	URL         string `validate:"required,url" query:"url"`
	Title       string `validate:"required" query:"title"`
	Description string `query:"description"`
	Image       string `query:"image"`
}

func (i *Inputs) Validate() error {
	return validate.Struct(i)
}
