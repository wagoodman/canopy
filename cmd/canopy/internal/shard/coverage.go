package shard

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	"golang.org/x/tools/cover"
)

// MergeCoverProfiles unions coverprofiles by block position. Counts are OR-ed in set mode and
// summed in count/atomic mode. Profiles in different modes are an error.
func MergeCoverProfiles(paths []string) ([]*cover.Profile, error) {
	var mode string
	byFile := map[string]*cover.Profile{}
	for _, path := range paths {
		profiles, err := cover.ParseProfiles(path)
		if err != nil {
			return nil, fmt.Errorf("unable to parse coverage profile %q: %w", path, err)
		}
		for _, p := range profiles {
			if mode == "" {
				mode = p.Mode
			}
			if p.Mode != mode {
				return nil, fmt.Errorf("coverage mode mismatch: %q is %q, expected %q", path, p.Mode, mode)
			}
			dst, ok := byFile[p.FileName]
			if !ok {
				dst = &cover.Profile{FileName: p.FileName, Mode: p.Mode}
				byFile[p.FileName] = dst
			}
			dst.Blocks = mergeBlocks(dst.Blocks, p.Blocks, mode == "set")
		}
	}

	out := make([]*cover.Profile, 0, len(byFile))
	for _, p := range byFile {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FileName < out[j].FileName })
	return out, nil
}

func mergeBlocks(dst, src []cover.ProfileBlock, set bool) []cover.ProfileBlock {
	type key struct{ sl, sc, el, ec int }
	idx := make(map[key]int, len(dst))
	for i, b := range dst {
		idx[key{b.StartLine, b.StartCol, b.EndLine, b.EndCol}] = i
	}
	for _, b := range src {
		k := key{b.StartLine, b.StartCol, b.EndLine, b.EndCol}
		i, ok := idx[k]
		if !ok {
			idx[k] = len(dst)
			dst = append(dst, b)
			continue
		}
		if set {
			if b.Count > 0 {
				dst[i].Count = 1
			}
		} else {
			dst[i].Count += b.Count
		}
	}
	sort.Slice(dst, func(i, j int) bool {
		a, b := dst[i], dst[j]
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.StartCol != b.StartCol {
			return a.StartCol < b.StartCol
		}
		if a.EndLine != b.EndLine {
			return a.EndLine < b.EndLine
		}
		return a.EndCol < b.EndCol
	})
	return dst
}

// WriteCoverProfile writes profiles in standard coverprofile format (mode line, then blocks).
func WriteCoverProfile(path string, profiles []*cover.Profile) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriter(f)
	mode := "set"
	if len(profiles) > 0 {
		mode = profiles[0].Mode
	}
	fmt.Fprintf(w, "mode: %s\n", mode)
	for _, p := range profiles {
		for _, b := range p.Blocks {
			fmt.Fprintf(w, "%s:%d.%d,%d.%d %d %d\n", p.FileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt, b.Count)
		}
	}
	return w.Flush()
}

// CoveragePercent is covered statements over total statements, matching the `go tool cover -func` total.
func CoveragePercent(profiles []*cover.Profile) float64 {
	var covered, total int
	for _, p := range profiles {
		for _, b := range p.Blocks {
			total += b.NumStmt
			if b.Count > 0 {
				covered += b.NumStmt
			}
		}
	}
	if total == 0 {
		return 0
	}
	return 100 * float64(covered) / float64(total)
}
