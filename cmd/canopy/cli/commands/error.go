package commands

type SilentError interface {
	error
	IsSilent() bool
}
