package ide

import (
	"fmt"
	"os/exec"
)

type Zed struct {
	binPath string
}

func NewZed(lookPathFunc func(string) (string, error)) (*Zed, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("zed")
	if err != nil {
		return nil, fmt.Errorf("unable to find zed binary: %w", err)
	}
	return &Zed{
		binPath: path,
	}, nil
}

func (z Zed) isActive(env EnvironmentGetter) bool {
	if env.Getenv("__CFBundleIdentifier") == "dev.zed.Zed" {
		return true
	}
	if env.Getenv("ZED_TERM") == "true" {
		return true
	}
	return false
}

func (z Zed) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf(`%s "%s:%d:0"`, z.binPath, filePath, line)
}

func (z Zed) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
