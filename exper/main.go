package main

import (
	"fmt"
	"time"
)

func mainold() {
	ch := make(chan int)
	go func() {
		fmt.Println("Pries")
		fmt.Printf("%v\n", <-ch)
		fmt.Println("Po")
	}()
	time.Sleep(5 * time.Second)
	//ch <- 314
	//fmt.Println("Irasyta")
	close(ch)
	fmt.Println("Uzdaryta")
	time.Sleep(5 * time.Second)
	fmt.Println("Pabaiga")
}
