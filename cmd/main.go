package main

import (
	"cloud.ru_test/app"
)

func main() {
	err := app.Run("config.yaml", ":8080")
	if err != nil {
		panic(err)
	}
}
