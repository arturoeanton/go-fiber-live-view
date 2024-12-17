package view

import (
	"fmt"
	"log"
)

func HandleRecover() {
	if r := recover(); r != nil {
		log.Println("Recovered from error:", r)
	}
}

func HandleRecoverMsg(msg string) {
	if r := recover(); r != nil {
		fmt.Println(msg, ":", r)
	}
}

func HandleRecoverPass() {
	recover()
}
