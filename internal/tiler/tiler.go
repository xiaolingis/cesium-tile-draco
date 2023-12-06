package tiler

type ITiler interface {
	RunTiler(opts *TilerOptions) error
}
