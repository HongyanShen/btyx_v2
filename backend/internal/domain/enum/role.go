package enum

// Role defines the allowed wire values for this business concept.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

func (Role) Values() []string {
	return []string{
		string(RoleAdmin),
		string(RoleUser),
	}
}

func (value Role) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *Role) UnmarshalJSON(data []byte) error {
	return decode(data, value, (Role("")).Values())
}
