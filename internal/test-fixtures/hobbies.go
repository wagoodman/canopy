package test_fixtures

type Person struct {
	Name    string
	Age     int
	Hobbies []string
}

func getCommonHobbies(people []Person) []string {
	if len(people) == 0 {
		// if the slice is empty, return an empty slice
		return []string{}
	}

	// create a map to store the hobbies and their frequency
	hobbyCounts := make(map[string]int)

	// iterate over the hobbies of the first person in the slice and add them to the hobbyCounts map
	for _, hobby := range people[0].Hobbies {
		hobbyCounts[hobby]++
	}

	// iterate over each of the remaining people and update the hobbyCounts map accordingly
	for _, person := range people[1:] {
		personHobbies := make(map[string]bool)
		for _, hobby := range person.Hobbies {
			personHobbies[hobby] = true
			// increment the count for any hobby that appears in both the current person and hobbyCounts
			if _, exists := hobbyCounts[hobby]; exists {
				hobbyCounts[hobby]++
			}
		}

		// decrement the count for any hobby that appears in hobbyCounts but not in the current person
		for hobby, count := range hobbyCounts {
			if !personHobbies[hobby] {
				hobbyCounts[hobby] = count - 1
			}
		}
	}

	// create a slice of strings to hold the common hobbies
	commonHobbies := make([]string, 0)

	// iterate over the hobbyCounts map and add any hobby with a count equal to the number of people in the slice to commonHobbies
	for hobby, count := range hobbyCounts {
		if count == len(people) {
			commonHobbies = append(commonHobbies, hobby)
		}
	}

	return commonHobbies
}
