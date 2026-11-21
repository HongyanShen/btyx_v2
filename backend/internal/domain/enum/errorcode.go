package enum

// ErrorCode defines the allowed wire values for this business concept.
type ErrorCode string

const (
	ErrorInvalidArgument ErrorCode = "INVALID_ARGUMENT"
	ErrorConflict        ErrorCode = "CONFLICT"
	ErrorForbidden       ErrorCode = "FORBIDDEN"
	ErrorNotFound        ErrorCode = "NOT_FOUND"
	ErrorUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrorInternal        ErrorCode = "INTERNAL_ERROR"
	ErrorUnavailable     ErrorCode = "UNAVAILABLE"
)

func (ErrorCode) Values() []string {
	return []string{
		string(ErrorInvalidArgument),
		string(ErrorConflict),
		string(ErrorForbidden),
		string(ErrorNotFound),
		string(ErrorUnauthorized),
		string(ErrorInternal),
		string(ErrorUnavailable),
	}
}

func (value ErrorCode) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *ErrorCode) UnmarshalJSON(data []byte) error {
	return decode(data, value, (ErrorCode("")).Values())
}
