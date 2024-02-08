// This example demonstrates a priority queue built using the heap interface.
package sim

import (
	"container/heap"
	"fmt"
	"testing"
)

// TestPQueue creates a PriorityQueue with some items, adds and
// manipulates an item, and then removes the items in priority order.
func TestPQueue(_ *testing.T) {
	// Some items and their priorities.
	items := map[string]int{
		"banana": 3, "apple": 2, "pear": 4,
	}

	// Create a priority queue, put the items in it, and
	// establish the priority queue (heap) invariants.
	pq := make(PQueue, len(items))
	i := 0

	for value, priority := range items {
		pq[i] = &Item{
			value:    value,
			priority: priority,
			index:    i,
		}
		i++
	}

	heap.Init(&pq)

	// Insert a new item and then modify its priority.
	item := &Item{
		value:    "orange",
		priority: 1,
	}
	heap.Push(&pq, item)

	val, ok := item.value.(string)
	if !ok {
		panic("type conversion failed in test")
	}

	pq.update(item, val, 5)

	// Take the items out; they arrive in decreasing priority order.
	for pq.Len() > 0 {
		item, ok := heap.Pop(&pq).(*Item)
		if !ok {
			panic("type conversion failed in test 2")
		}

		fmt.Printf("%.2d:%s ", item.priority, item.value)
	}
}
