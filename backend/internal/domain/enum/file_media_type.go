package enum

type FileMediaType string

const (
	FileJPEG FileMediaType = "image/jpeg"
	FilePNG  FileMediaType = "image/png"
	FileGIF  FileMediaType = "image/gif"
	FileWEBP FileMediaType = "image/webp"
	FileMP4  FileMediaType = "video/mp4"
)

func (FileMediaType) Values() []string {
	return []string{string(FileJPEG), string(FilePNG), string(FileGIF), string(FileWEBP), string(FileMP4)}
}

func (value FileMediaType) Valid() bool {
	return valid(string(value), value.Values())
}

func (value *FileMediaType) UnmarshalJSON(data []byte) error {
	return decode(data, value, (FileMediaType("")).Values())
}
