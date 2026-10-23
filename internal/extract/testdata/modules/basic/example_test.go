package acme_test

import (
	"fmt"

	"example.com/acme"
)

func ExampleNewClient() {
	client := acme.NewClient()
	fmt.Println(client.BaseURL == "")
	// Output: true
}

func ExampleClient_Do() {
	fmt.Println("done")
}
