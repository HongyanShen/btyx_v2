package enum

type FileStatus string

const (
	FilePending FileStatus = "pending"
	FileReady   FileStatus = "ready"
)

func (FileStatus) Values() []string {
	return []string{string(FilePending), string(FileReady)}
}

func (value FileStatus) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *FileStatus) UnmarshalJSON(data []byte) error {
	return decode(data, value, (FileStatus("")).Values())
}
