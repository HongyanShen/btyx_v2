package enum

type FileStorage string

const (
	FileStorageLocal FileStorage = "local"
	FileStorageTOS   FileStorage = "tos"
)

func (FileStorage) Values() []string {
	return []string{string(FileStorageLocal), string(FileStorageTOS)}
}

func (value FileStorage) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *FileStorage) UnmarshalJSON(data []byte) error {
	return decode(data, value, (FileStorage("")).Values())
}
