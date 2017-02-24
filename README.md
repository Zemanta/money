# money.Micro

[![CircleCI](https://circleci.com/gh/Zemanta/money/tree/master.svg?style=svg)](https://circleci.com/gh/Zemanta/money/tree/master)

A precise fixed-point Go (golang) package for handling money as int64 supporting up to 6 decimal places. Useful when handling smaller amounts but need percision and fast math operations.

## Features

* Speed and simplicity
* Support for safe math operations with overflow checking: addition, multiplication and division with rounding
* JSON marshal/unmarshal
* Conversion from string and float64
* Works with integer operations: <, >, ==, etc.

## Install

Run `go get github.com/Zemanta/money`

## Usage

```go
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
```
