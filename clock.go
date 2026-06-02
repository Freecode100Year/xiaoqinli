package main

import (
	"fmt"
	"time"
)

func main() {
    fmt.Println("=============================")
    fmt.Println("      System Clock")
    fmt.Println("=============================")
    for true {
        now := time.Now().Format("2006-01-02 15:04:05")
        fmt.Printf("\r  [%s]  ", now)
        time.Sleep(time.Second)
    }
}

