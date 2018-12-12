// You can edit this code!
// Click here and start typing.
package main

import "log"

func main() {
	if error := Demo(); error != nil {
		log.Fatalf(error.Error())
	}
}
