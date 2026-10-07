package response

import "testing"

// Field names reach clients in validation errors, so a mangled one makes the
// error impossible to attribute to the field the caller sent.
func TestSnakeCase(t *testing.T) {
	cases := map[string]string{
		"DeliveryAddressID": "delivery_address_id",
		"UserID":            "user_id",
		"ID":                "id",
		"IDToken":           "id_token",
		"Name":              "name",
		"FirstName":         "first_name",
		"SKUCode":           "sku_code",
		"HTTPPort":          "http_port",
		"BasePrice":         "base_price",
		"":                  "",
		"email":             "email",
	}
	for in, want := range cases {
		if got := snakeCase(in); got != want {
			t.Errorf("snakeCase(%q) = %q, want %q", in, got, want)
		}
	}
}
