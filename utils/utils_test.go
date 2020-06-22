package utils

import (
	"fmt"
	"testing"
)

func TestTimeEpochFormat(t *testing.T) {
	dateAdded := int64(13220074277028707)
	s := TimeEpochFormat(dateAdded)
	fmt.Println(s)
}
