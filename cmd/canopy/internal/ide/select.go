package ide

func Select(env EnvironmentGetter) Context {
	var available []Context
	if c, err := NewZed(nil); err == nil {
		available = append(available, c)
	}

	if c, err := NewGoland(nil); err == nil {
		available = append(available, c)
	}

	if c, err := NewVSCode(nil); err == nil {
		available = append(available, c)
	}

	for _, c := range available {
		if c.isActive(env) {
			return c
		}
	}

	return &dummy{}
}

type dummy struct {
}

func (d dummy) isActive(_ EnvironmentGetter) bool {
	return true
}

func (d dummy) OpenFileAtLineCommand(_ string, _ int) string {
	return ""
}

func (d dummy) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
