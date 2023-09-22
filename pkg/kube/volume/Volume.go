package volume

type Source int

const (
	Secret Source = iota
	ConfigMap
)

type Volume struct {
	Kind       Source
	Name       string
	SourceName string
}
