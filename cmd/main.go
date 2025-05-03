package main

import (
	"fmt"

	"github.com/bermr/api-golang-base/configs"
)

func main() {
	config, err := configs.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	fmt.Printf("Config loaded sucessfully: %+v\n", config)
}
