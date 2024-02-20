package main

import "log"

func main() {
	config := readConfig("config.yaml") // call the readConfig function of config.go
	runBot(config)                      // Run the bot passing in required arguments
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message is: ", e)
	}
}
