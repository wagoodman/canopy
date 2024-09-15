package ide

import (
	"fmt"
	"os/exec"
)

var _ Context = (*Goland)(nil)

type Goland struct {
	binPath string
}

func NewGoland(lookPathFunc func(string) (string, error)) (*Goland, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("goland")
	if err != nil {
		return nil, fmt.Errorf("unable to find goland binary: %w", err)
	}
	return &Goland{
		binPath: path,
	}, nil
}

func (g Goland) isActive(env EnvironmentGetter) bool {
	if env.Getenv("__CFBundleIdentifier") == "com.jetbrains.goland" {
		return true
	}

	if env.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm" {
		return true
	}
	return false
}

func (g Goland) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf("%s --line %d %s", g.binPath, line, filePath)
}

func (g Goland) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
