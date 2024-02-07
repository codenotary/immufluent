package delaybuffer

import (
	"log"
	"time"
)

type delaybuffer[T any] struct {
	payload   []T
	ch        chan T
	timerStep time.Duration
	sendFunc  func([]T) error
	batchsize int
}

func NewDelayBuffer[T any](batchsize int, step time.Duration, sendF func([]T) error) *delaybuffer[T] {
	db := delaybuffer[T]{
		ch:        make(chan T, batchsize*2),
		batchsize: batchsize,
		timerStep: step,
		sendFunc:  sendF,
	}
	go db.loop()
	return &db
}

func (db *delaybuffer[T]) loop() {
	t := time.NewTimer(db.timerStep)
	for {
		select {
		case m := <-db.ch:
			db.payload = append(db.payload, m)
			if len(db.payload) == db.batchsize {
				log.Printf("Batchsize reached")
				t.Stop()
				db.doSend()
			} else {
				t.Reset(db.timerStep)
			}
		case <-t.C:
			if len(db.payload) > 0 {
				log.Printf("Timer writing %d", len(db.payload))
				db.doSend()
			} else {
				log.Printf("Empty timer triggered")
			}
		}
	}
}

func (db *delaybuffer[T]) doSend() {
	if err := db.sendFunc(db.payload); err != nil {
		log.Printf("Error while sending: %s", err.Error())
	}
	db.payload = nil
}

func (db *delaybuffer[T]) Push(msg T) {
	db.ch <- msg
}
