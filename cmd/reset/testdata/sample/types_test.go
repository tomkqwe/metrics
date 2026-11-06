package sample

import "testing"

func TestReset(t *testing.T) {
	n := 9
	p := &n
	slice := []int{3, 4}
	m := map[string]int{"a": 1}
	child := &Item{Text: "child"}
	item := &Item{
		Number: 7, Text: "text", Flag: true, Float: 1.5, Complex: 2i,
		Ptr: p, Double: &p, Slice: make([]string, 2, 8), Map: m,
		SlicePtr: &slice, MapPtr: &m, Child: child,
		Custom: Custom{8}, Plain: Plain{5}, Array: [2]int{1, 2},
		Callback: func() {}, Channel: make(chan int), Interface: "value",
	}
	backing := &item.Slice[0]
	item.Reset()
	if item.Number != 0 || item.Text != "" || item.Flag || item.Float != 0 || item.Complex != 0 {
		t.Fatal("primitive fields were not reset")
	}
	if n != 0 || item.Ptr != p || item.Double != &p {
		t.Fatal("pointers were not retained or reset")
	}
	if len(item.Slice) != 0 || cap(item.Slice) != 8 || &item.Slice[:1][0] != backing {
		t.Fatal("slice allocation was not retained")
	}
	if item.Map == nil || len(m) != 0 || len(slice) != 0 || cap(slice) != 2 || item.SlicePtr != &slice || item.MapPtr != &m {
		t.Fatal("collections were not cleared in place")
	}
	if item.Child != child || child.Text != "" || item.Custom.Value != -1 || item.Plain.Keep != 5 {
		t.Fatal("nested reset semantics are incorrect")
	}
	if item.Array != [2]int{} || item.Callback != nil || item.Channel != nil || item.Interface != nil {
		t.Fatal("remaining fields were not reset")
	}
	item.Reset()
	var empty Item
	empty.Reset()
	if empty.Ptr != nil || empty.Slice != nil || empty.Map != nil || empty.Child != nil {
		t.Fatal("nil fields should remain nil")
	}
	var nilItem *Item
	nilItem.Reset()
	box := Box[int]{Value: 42, Items: make([]int, 2, 4)}
	box.Reset()
	if box.Value != 0 || len(box.Items) != 0 || cap(box.Items) != 4 {
		t.Fatal("generic reset failed")
	}
	embedded := Embedded{Custom{5}}
	embedded.Reset()
	if embedded.Value != -1 {
		t.Fatal("embedded field was not reset")
	}
}
