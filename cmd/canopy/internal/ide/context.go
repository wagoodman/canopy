package ide

import "fmt"

type Context interface {
	isActive(env EnvironmentGetter) bool
	OpenFileAtLineCommand(file string, line int) string
	FileAtLineURL(file string, line int) string
}

func fileAtLineURL(file string, line int) string {
	return fmt.Sprintf("file://%s:%d", file, line)
}
