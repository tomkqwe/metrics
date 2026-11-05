package main

import (
	"log"
	"os"
)

func main() {
	log.Fatal("allowed")
	log.Fatalf("%s", "allowed")
	log.Fatalln("allowed")
	if true {
		os.Exit(0)
	}
	func() {
		os.Exit(1)         // want "os.Exit is forbidden outside main.main"
		log.Fatal("error") // want "log.Fatal is forbidden outside main.main"
	}()
	// Leaving the closure restores the enclosing main context.
	os.Exit(0)
	panic("error") // want "use of builtin panic is forbidden"
}

func init() {
	os.Exit(1) // want "os.Exit is forbidden outside main.main"
}

func helper() {
	log.Fatal("error") // want "log.Fatal is forbidden outside main.main"
}

type worker struct{}

func (worker) main() {
	os.Exit(1) // want "os.Exit is forbidden outside main.main"
}
