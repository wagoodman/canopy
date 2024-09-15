package ide

import (
	"fmt"
	"os/exec"
)

type VSCode struct {
	binPath string
}

func NewVSCode(lookPathFunc func(string) (string, error)) (*VSCode, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("code")
	if err != nil {
		return nil, fmt.Errorf("unable to find code binary: %w", err)
	}
	return &VSCode{
		binPath: path,
	}, nil
}

func (v VSCode) isActive(env EnvironmentGetter) bool {
	return env.Getenv("TERM_PROGRAM") == "vscode"
}

func (v VSCode) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf(`%s --goto "%s:%d"`, v.binPath, filePath, line)
}

func (v VSCode) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
