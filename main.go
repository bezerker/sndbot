package main

func main() {
	config := readConfig("config.yaml") // call the readConfig function of config.go
	runBot(config)                      // Run the bot passing in required arguments
}
