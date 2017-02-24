package main

import (
	"fmt"
	"log"

	"github.com/Zemanta/money"
)

func main() {
	balance := 100 * money.Dollar

	m, err := money.FromString("0.1")
	if err != nil {
		log.Fatal("Error")
	}

	newBalance := balance - m
	if newBalance < 0 {
		log.Fatal("Not enough money")
	}

	s, _ := money.ToString(newBalance)
	fmt.Println(s)
}
