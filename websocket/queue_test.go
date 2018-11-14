package websocket

import (
	"fmt"
	"golang.org/x/net/context"
	"sync"
	"testing"
	"time"
)

var jobChan chan int
var wg sync.WaitGroup

func work(jobChan <- chan int)  {
	defer wg.Done()
	for job := range jobChan{
		//fmt.Println("---------")
		fmt.Printf("执行任务 %d \n", job)
	}
}
var ctx context.Context
var cancel context.CancelFunc
func Test(t *testing.T) {
	jobChan = make(chan int, 100)
	//带有取消功能的 contex
	ctx, cancel = context.WithCancel(context.Background())
	//入队
	for i := 1; i <= 10; i++{
		jobChan <- i
	}
	wg.Add(1)
	close(jobChan)
	go work(jobChan)
	//time.Sleep(2 * time.Second)
	//調用cancel
	//cancel()
	res := waitTimeout(&wg, 1 * time.Second)
	if res {
		fmt.Println("执行完成退出")
	} else {
		fmt.Println("执行超时")
	}

}
//超时机制
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}