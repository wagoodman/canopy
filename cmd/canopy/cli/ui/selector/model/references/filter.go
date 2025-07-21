package references

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"sort"
)

func init() {
	algo.Init("path")
}

// rankResult holds the result for a single target
type rankResult struct {
	Score int
	list.Rank
}

type byScore []rankResult

func (a byScore) Len() int           { return len(a) }
func (a byScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byScore) Less(i, j int) bool { return a[i].Score > a[j].Score } // Higher score first

// filter adapts fzf's FuzzyMatchV2 to work with bubbletea's list.Rank interface, which seems to work much
// better than the default fuzzy matching provided by bubbletea's list package (for instance, when providing
// search terms that look like paths, such as "foo/bar/baz" then the selected characters tend match the
// relevant path segments, rather than just the first character of each segment).
func filter(term string, targets []string) []list.Rank {
	if term == "" {
		// return all items with original indices when no filter term
		result := make([]list.Rank, len(targets))
		for i := range targets {
			result[i] = list.Rank{
				Index:          i,
				MatchedIndexes: nil,
			}
		}
		return result
	}

	pattern := []rune(term)
	var slab util.Slab
	var results []rankResult

	for i, target := range targets {
		chars := util.ToChars([]byte(target))
		result, positions := algo.FuzzyMatchV2(
			false, // consider input as case-insensitive
			true,  // non ascii characters are equated to similar ascii characters
			true,  // forward search
			&chars,
			pattern,
			true, // keep positions
			&slab,
		)

		if result.Start >= 0 { // Valid match
			var matchedIndexes []int
			if positions != nil {
				matchedIndexes = make([]int, len(*positions))
				for j, pos := range *positions {
					matchedIndexes[j] = int(pos)
				}
			}

			results = append(results, rankResult{
				Rank: list.Rank{
					Index:          i,
					MatchedIndexes: matchedIndexes,
				},
				Score: result.Score,
			})
		}
	}

	// sort by score (highest first)
	sort.Sort(byScore(results))

	// convert to list.Rank format
	ranks := make([]list.Rank, len(results))
	for i, r := range results {
		ranks[i] = r.Rank
	}

	return ranks
}
