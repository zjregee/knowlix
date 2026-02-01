package models

type ApiItem struct {
	ItemID            string
	Kind              string
	Name              string
	Signature         string
	Package           string
	ImportPath        string
	Receiver          string
	Params            string
	Returns           string
	TypeKind          string
	Fields            []string
	Methods           []string
	SourceDescription string
}

type GeneratedDoc struct {
	Item        ApiItem
	Content     string
	GeneratedAt string
	Generator   string
	Model       string
}
