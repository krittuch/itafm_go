package main

import (
	"aerothai/itafm/app"
)

func main() {
	var a app.App

	a.CreateConnection()

	a.Run()
}
