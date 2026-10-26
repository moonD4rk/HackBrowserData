package main

import (
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/category"
)

// parseCategories converts a comma-separated string into a Category slice.
// "all" returns all categories.
func parseCategories(s string) ([]category.Category, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "all") {
		return category.All(), nil
	}

	var categories []category.Category
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		c, err := category.Parse(name)
		if err != nil {
			return nil, fmt.Errorf("%w, available: all|%s", err, categoryNames())
		}
		categories = append(categories, c)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories specified")
	}
	return categories, nil
}

func categoryNames() string {
	all := category.All()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = c.String()
	}
	return strings.Join(names, ",")
}
