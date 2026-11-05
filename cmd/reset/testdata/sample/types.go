package sample

type Count int
type Plain struct{ Keep int }
type Custom struct{ Value int }

func (c *Custom) Reset() { c.Value = -1 }

// generate:reset
type Item struct {
	Number    Count
	Text      string
	Flag      bool
	Float     float64
	Complex   complex128
	Ptr       *int
	Double    **int
	Slice     []string
	Map       map[string]int
	SlicePtr  *[]int
	MapPtr    *map[string]int
	Child     *Item
	Custom    Custom
	Plain     Plain
	Array     [2]int
	Callback  func()
	Channel   chan int
	Interface any
	_         int
}

type (
	// generate:reset
	Box[T any] struct {
		Value T
		Items []T
	}
	Ignored struct{ Value int }
)

// generate:reset
type Embedded struct{ Custom }
