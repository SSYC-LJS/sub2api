package admin

// stringSetting keeps partial PUT updates from replacing an existing string
// setting when the field is omitted from the request payload.
func stringSetting(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
