package sample

import (
	logger "log"
	process "os"
	"testing"
)

var initialized = func() int {
	process.Exit(1) // want "os.Exit is forbidden outside main.main"
	return 0
}()

func main() {
	logger.Fatal("error") // want "log.Fatal is forbidden outside main.main"
	process.Exit(1)       // want "os.Exit is forbidden outside main.main"
}

func bad() {
	panic("error")               // want "use of builtin panic is forbidden"
	(panic)("error")             // want "use of builtin panic is forbidden"
	logger.Fatalf("%s", "error") // want "log.Fatalf is forbidden outside main.main"
	logger.Fatalln("error")      // want "log.Fatalln is forbidden outside main.main"
	(process.Exit)(1)            // want "os.Exit is forbidden outside main.main"
	defer logger.Fatal("error")  // want "log.Fatal is forbidden outside main.main"
	go process.Exit(1)           // want "os.Exit is forbidden outside main.main"
	func() {
		panic("error")  // want "use of builtin panic is forbidden"
		process.Exit(1) // want "os.Exit is forbidden outside main.main"
	}()
}

func allowed(t *testing.T) {
	panic := func(any) {}
	panic("custom function")
	log := struct{ Fatal func(...any) }{func(...any) {}}
	log.Fatal("custom field")
	os := struct{ Exit func(int) }{func(int) {}}
	os.Exit(0)
	t.Fatal("testing method")
	logger.Print("message")
}
