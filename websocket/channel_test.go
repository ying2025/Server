package websocket

import (
	"fmt"
	"testing"
	"time"
)

func Test5(t *testing.T) {
	// Start the server:
	serverChan:= make(chan chan string, 4)
	go server(serverChan)

	// Connect the clients:
	client1Chan := make(chan string, 4)
	client2Chan := make(chan string, 4)
	client3Chan := make(chan string, 4)
	client4Chan := make(chan string, 4)
	client5Chan := make(chan string, 4)
	serverChan <- client1Chan
	serverChan <- client2Chan
	serverChan <- client3Chan
	serverChan <- client4Chan
	serverChan <- client5Chan

	// Notice that we have to start the clients in their own goroutine,// because we would have a deadlock otherwise:
	go client("Client 1", client1Chan)
	go client("Client 2", client2Chan)
	go client("Client 3", client3Chan)
	go client("Client 4", client4Chan)
	go client("Client 5", client5Chan)
	// Just a dirty hack to wait for everything to finish up.
	// A clean and safe approach would have been too much boilerplate code
	// for this blog-post
	time.Sleep(time.Second)
}

func server(serverChan chan chan string) {
	var clients []chan string
	for {
		select {
		case client, _ := <-serverChan:
			clients = append(clients, client)
			// Broadcast the number of clients to all clients:
			for _, c := range clients {
				c <- fmt.Sprintf("%d client(s) connected.", len(clients))
			}
		}
	}
}

func client(clientName string, clientChan chan string) {
	for {
		text, _ := <-clientChan
		fmt.Printf("%s: %s\n", clientName, text)
	}
}

func uptimeServer(serverChan chan chan string) {
	var clients []chan string
	uptimeChan := make(chan int, 1)
	// This goroutine will count our uptime in the background, and write
	// updates to uptimeChan:
	go func (target chan int) {
		i := 0
		for {
			time.Sleep(time.Second)
			i++
			target <- i
		}
	}(uptimeChan)
	// And now we listen to new clients and new uptime messages:
	for {
		select {
		case client, _ := <-serverChan:
			clients = append(clients, client)
		case uptime, _ := <-uptimeChan:
			// Send the uptime to all connected clients:
			for _, c := range clients {
				c <- fmt.Sprintf("%d seconds uptime", uptime)
			}
		}
	}
}

// Here's the worker, of which we'll run several
// concurrent instances. These workers will receive
// work on the `jobs` channel and send the corresponding
// results on `results`. We'll sleep a second per job to
// simulate an expensive task.
func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		fmt.Println("worker", id, "started  job", j)
		time.Sleep(time.Second)
		fmt.Println("worker", id, "finished job", j)
		results <- j * 2
	}
}


func TestWorker(t *testing.T) {

	// In order to use our pool of workers we need to send
	// them work and collect their results. We make 2
	// channels for this.
	jobs := make(chan int, 100)
	results := make(chan int, 100)

	// This starts up 3 workers, initially blocked
	// because there are no jobs yet.
	for w := 1; w <= 3; w++ {
		go worker(w, jobs, results)
	}

	// Here we send 5 `jobs` and then `close` that
	// channel to indicate that's all the work we have.
	for j := 1; j <= 5; j++ {
		jobs <- j
	}
	close(jobs)

	// Finally we collect all the results of the work.
	for a := 1; a <= 5; a++ {
		fmt.Println("receive the result", <-results)
	}
}

