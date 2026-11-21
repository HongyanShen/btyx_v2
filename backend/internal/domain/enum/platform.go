package enum

// Platform defines the allowed wire values for this business concept.
type Platform string

const (
	PlatformDouyin      Platform = "dy"
	PlatformXiaohongshu Platform = "xhs"
)

func (Platform) Values() []string {
	return []string{
		string(PlatformDouyin),
		string(PlatformXiaohongshu),
	}
}

func (value Platform) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *Platform) UnmarshalJSON(data []byte) error {
	return decode(data, value, (Platform("")).Values())
}
