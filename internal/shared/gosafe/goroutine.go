package goroutine

import (
	"fmt"
	"sync"
)

// RunGo menjalankan kumpulan fungsi dalam goroutine dengan recover & error channel.
func Go(funcs ...func() error) <-chan error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(funcs))

	for _, fn := range funcs {
		wg.Add(1)
		go func(fn func() error) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic recovered: %v", r)
				}
			}()

			if err := fn(); err != nil {
				errChan <- err
			}
		}(fn)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	return errChan
}
