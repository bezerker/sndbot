package util

import "log"

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message is: ", e)
	}
}
