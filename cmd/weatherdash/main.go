// Command weatherdash is a terminal dashboard for comparing weather
// across a saved list of cities.
package main

import (
	"fmt"

	"github.com/tsoueid/weatherdash/internal/version"
)

func main() {
	fmt.Printf("weatherdash %s\n", version.Version)
}
