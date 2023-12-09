package gtask

import (
	"testing"
	"time"
)

func TestGTask(t *testing.T) {
	t.Log("in TestGTask")

	Init("config.json")

	go func() {
		time.Sleep(2 * time.Second)
		Stop()
	}()

	Run()

	t.Log("end TestGTask")
}
