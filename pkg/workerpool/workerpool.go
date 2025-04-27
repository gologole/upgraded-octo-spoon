package workerpool

import "sync"

type WorkerPool struct {
	taskQueue chan func()
	wg        sync.WaitGroup
}

func NewWorkerPool(workerCount int) *WorkerPool {
	pool := &WorkerPool{
		taskQueue: make(chan func()),
	}
	for i := 0; i < workerCount; i++ {
		go pool.worker()
	}
	return pool
}

func (wp *WorkerPool) worker() {
	for task := range wp.taskQueue {
		task()
	}
}

func (wp *WorkerPool) Submit(task func()) {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		wp.taskQueue <- task
	}()
}

func (wp *WorkerPool) Wait() {
	close(wp.taskQueue)
	wp.wg.Wait()
}
