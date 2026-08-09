package releaseinfo

//go:generate go run ./generate

type Document struct {
	Version string
	Summary string
	Content string
}

func Releases() []Document {
	return append([]Document(nil), releases...)
}

func Roadmap() []Document {
	return append([]Document(nil), roadmap...)
}
