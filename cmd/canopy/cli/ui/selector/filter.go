package selector

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

func init() {
	algo.Init("path")
}

// rankResult holds the fuzzy match result for a single target string.
// it combines the score from fzf with the list.Rank structure.
type rankResult struct {
	// Score is the fuzzy match score from fzf (higher is better).
	Score int
	list.Rank
}

// byScore implements sort.Interface to sort rank results by score descending.
type byScore []rankResult

func (a byScore) Len() int           { return len(a) }
func (a byScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byScore) Less(i, j int) bool { return a[i].Score > a[j].Score } // Higher score first

// filter adapts fzf's FuzzyMatchV2 to work with bubbletea's list.Rank interface.
// it provides path-aware fuzzy matching that works better than the default list filtering,
// especially for search terms that look like paths (e.g., "foo/bar/baz").
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

	term = strings.ToLower(strings.TrimSpace(term))

	pattern := []rune(term)
	var slab util.Slab
	var results []rankResult

	for i, target := range targets {
		chars := util.ToChars([]byte(target))
		result, positions := algo.FuzzyMatchV2(
			false, // consider input as case-insensitive
			true,  // non ascii characters are equated to similar ascii characters
			false, // backwards search will bias the test and cases over the directory structure
			&chars,
			pattern,
			true, // keep positions
			&slab,
		)

		if result.Start >= 0 { // Valid match
			var matchedIndexes []int
			if positions != nil {
				matchedIndexes = make([]int, len(*positions))
				copy(matchedIndexes, *positions)
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
