package snowflake

import (
	"sync"
	"testing"
)

func TestDistinctNodesAndConcurrentIDs(t *testing.T) {
	var workers sync.WaitGroup
	ids := make(chan int64, 400)
	for _, node := range []int64{1, 1023} {
		generator, err := New(node)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				for range 100 {
					ids <- generator.Next()
				}
			}()
		}
	}
	workers.Wait()
	close(ids)
	seen := make(map[int64]bool)
	for id := range ids {
		if id <= 0 || seen[id] {
			t.Fatalf("生成了无效或重复的 ID %d", id)
		}
		seen[id] = true
	}
	if _, err := New(1024); err == nil {
		t.Fatal("节点编号超出范围时应返回错误")
	}
}
